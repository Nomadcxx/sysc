package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/Nomadcxx/sysc/internal/conflict"
	"github.com/Nomadcxx/sysc/internal/fetch"
	"github.com/Nomadcxx/sysc/internal/geo"
	"github.com/Nomadcxx/sysc/internal/i18n"
	"github.com/Nomadcxx/sysc/internal/install"
	"github.com/Nomadcxx/sysc/internal/pin"
	"github.com/Nomadcxx/sysc/internal/preflight"
	"github.com/Nomadcxx/sysc/internal/seed"
	"github.com/Nomadcxx/sysc/internal/ui"
	"github.com/Nomadcxx/sysc/internal/units"
)

type options struct {
	Yes            bool
	City           string
	Lat            float64
	Lon            float64
	Lang           string
	Purge          bool
	RemoveGSlapper bool
	KeepConflicts  bool
	Handover       string
}

func parseFlags(args []string, out io.Writer) (options, error) {
	var o options
	fs := flag.NewFlagSet("sysc", flag.ContinueOnError)
	fs.SetOutput(out)
	fs.Usage = func() {
		fmt.Fprintln(out, "Usage: sysc [flags]            install")
		fmt.Fprintln(out, "       sysc uninstall [flags]  remove an installation")
		fs.PrintDefaults()
	}
	fs.BoolVar(&o.Yes, "yes", false, "install with defaults, no prompts")
	fs.StringVar(&o.City, "city", "", "weather city name")
	fs.Float64Var(&o.Lat, "lat", 0, "weather latitude")
	fs.Float64Var(&o.Lon, "lon", 0, "weather longitude")
	fs.StringVar(&o.Lang, "lang", "", "installer language (en, zh-Hans, de, fr, es, pt, ja, ko, ru)")
	fs.BoolVar(&o.KeepConflicts, "keep-conflicts", false, "keep every detected notification daemon and bar running")
	fs.StringVar(&o.Handover, "handover", "", "hand over conflicts: --handover=all")
	if err := fs.Parse(args); err != nil {
		return o, err
	}
	if fs.NArg() != 0 {
		return o, fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
	}
	if o.KeepConflicts && o.Handover != "" {
		return o, errors.New("--keep-conflicts and --handover are mutually exclusive")
	}
	if o.Handover != "" && o.Handover != "all" {
		return o, errors.New(`--handover only accepts "all"`)
	}
	return o, nil
}

func parseUninstall(args []string, out io.Writer) (options, error) {
	var o options
	fs := flag.NewFlagSet("sysc uninstall", flag.ContinueOnError)
	fs.SetOutput(out)
	fs.BoolVar(&o.Yes, "yes", false, "no prompts")
	fs.BoolVar(&o.Purge, "purge", false, "also remove user config")
	fs.BoolVar(&o.RemoveGSlapper, "remove-gslapper", false, "also remove gSlapper with the package manager")
	fs.StringVar(&o.Lang, "lang", "", "installer language (en, zh-Hans, de, fr, es, pt, ja, ko, ru)")
	if err := fs.Parse(args); err != nil {
		return o, err
	}
	if fs.NArg() != 0 {
		return o, fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
	}
	return o, nil
}

func runUninstall(args []string, in io.Reader, out io.Writer, home string) int {
	o, err := parseUninstall(args, out)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintln(out, err)
		return 2
	}
	if os.Geteuid() == 0 {
		fmt.Fprintln(out, "Run the installer as your desktop user, without sudo.")
		return 1
	}
	p, err := pin.Load()
	if err != nil {
		fmt.Fprintln(out, err)
		return 1
	}
	if !o.Yes {
		fmt.Fprintln(out, "This removes the SYSC suite binaries and user units.")
		if o.Purge {
			fmt.Fprintln(out, "The user config under ~/.config/sysc-shell is removed too (--purge).")
		}
		if o.RemoveGSlapper {
			fmt.Fprintln(out, "gSlapper is removed with the package manager (--remove-gslapper).")
		}
		fmt.Fprint(out, "Proceed? [y/N] ")
		if !confirm(in) {
			fmt.Fprintln(out, "Aborted.")
			return 1
		}
	}
	osr, _ := os.ReadFile("/etc/os-release")
	opts := installOptions(home, p, seed.Answers{}, o.Yes, i18n.Match(o.Lang), osr, runPackage)
	opts.Purge = o.Purge
	opts.RemoveGSlapper = o.RemoveGSlapper
	res, err := install.Uninstall(opts)
	printTasks(out, res)
	if err != nil {
		fmt.Fprintln(out, err)
		return 1
	}
	if anyFailed(res.Tasks) {
		fmt.Fprintln(out, "uninstall finished with failed tasks")
		return 1
	}
	return 0
}

func confirm(in io.Reader) bool {
	sc := bufio.NewScanner(in)
	if !sc.Scan() {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(sc.Text())) {
	case "y", "yes":
		return true
	}
	return false
}

func anyFailed(tasks []install.Task) bool {
	for _, t := range tasks {
		if t.Status == install.Failed {
			return true
		}
	}
	return false
}

func answersFor(ctx context.Context, o options, recommended []string) (seed.Answers, error) {
	a := seed.Answers{Preset: "standard", Mode: "dark", WallpaperDir: "~/Pictures/wallpapers", Plugins: recommended}
	switch {
	case o.City != "":
		p, err := geo.Search(ctx, nil, geo.DefaultGeocodeEndpoint, o.City)
		if err != nil {
			return a, err
		}
		a.Latitude, a.Longitude, a.Location = p.Latitude, p.Longitude, p.City
	case o.Lat != 0 || o.Lon != 0:
		if o.Lat == 0 || o.Lon == 0 {
			return a, errors.New("--lat and --lon must be given together")
		}
		a.Latitude, a.Longitude = o.Lat, o.Lon
		a.Location = fmt.Sprintf("%.4f, %.4f", o.Lat, o.Lon)
	default:
		p, err := geo.Guess(ctx, nil, geo.DefaultEndpoint)
		if err != nil {
			return a, fmt.Errorf("no weather location: pass --city or --lat/--lon: %w", err)
		}
		a.Latitude, a.Longitude, a.Location = p.Latitude, p.Longitude, p.City
	}
	return a, nil
}

func aurHelper() string {
	for _, h := range []string{"yay", "paru"} {
		if _, err := exec.LookPath(h); err == nil {
			return h
		}
	}
	return ""
}

func installOptions(home string, p pin.Pin, a seed.Answers, yes bool, loc i18n.Locale, osRelease []byte, run func(*exec.Cmd) error) install.Options {
	opts := install.Options{Home: home, Pin: p, Answers: a, Yes: yes, Loc: &loc, Client: fetch.NewClient(), CheckRuntime: preflight.CheckRuntime, CheckOwnership: install.CheckOwnership}
	wirePackages(&opts, osRelease, runtime.GOARCH, run)
	return opts
}

// The AUR helper builds as the user and elevates only its package-manager step.
func packageCommand(helper, action, pkg string, yes bool) *exec.Cmd {
	args := []string{action}
	if action == "-S" {
		args = append(args, "--needed")
	}
	if yes {
		args = append(args, "--noconfirm")
	}
	return exec.Command(helper, append(args, "--", pkg)...)
}

func runPackage(cmd *exec.Cmd) error {
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

type packageRequestMsg struct {
	cmd   *exec.Cmd
	reply chan error
}

type packageFinishedMsg struct{}

type tickMsg time.Time

type guessMsg struct {
	place geo.Place
	err   error
	query string
}

type progressMsg struct{ tasks []install.Task }

type installDoneMsg struct {
	res install.Result
	err error
}

type model struct {
	w            ui.Wizard
	beams        *ui.BeamsTextEffect
	input        textinput.Model
	width        int
	height       int
	step         ui.Step
	tasks        []install.Task
	installRes   install.Result
	installErr   error
	logPath      string
	frame        int
	note         string
	scroll       int
	lookingUp    bool
	weatherQuery string
	weatherNote  string

	// send is wired to the running tea program in main; installer is the
	// injectable seam for tests (never call the real install.Run in tests).
	send      func(tea.Msg)
	installer func(seed.Answers, func([]install.Task)) (install.Result, error)
}

func newModel(loc i18n.Locale, recommended []string, plainNiri bool) model {
	in := textinput.New()
	in.Placeholder = i18n.T(loc, "weather.place")
	in.CharLimit = 1024
	in.Width = ui.ContentWidth(ui.MinWidth) - 6
	in.Prompt = "› "
	in.TextStyle = lipgloss.NewStyle().Foreground(ui.White).Background(ui.Black)
	in.PromptStyle = in.TextStyle
	in.PlaceholderStyle = lipgloss.NewStyle().Foreground(ui.Muted).Background(ui.Black)
	in.Cursor.Style = in.TextStyle
	w := ui.NewWizard(loc, recommended)
	w.PlainNiri = plainNiri
	m := model{
		w:       w,
		input:   in,
		width:   ui.MinWidth,
		height:  ui.MinHeight,
		step:    ui.StepWizard,
		logPath: filepath.Join(stateHome(), "sysc", "installer.log"),
	}
	if os.Getenv("NO_COLOR") == "" && os.Getenv("SYSC_REDUCED_MOTION") == "" {
		m.beams = ui.NewBeamsTextEffect(min(m.width-4, 100), ui.BannerHeight(), ui.Banner())
	}
	return m
}

func (m model) Init() tea.Cmd { return tick() }

func tick() tea.Cmd {
	return tea.Tick(50*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func guessCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		p, err := geo.Guess(ctx, nil, geo.DefaultEndpoint)
		return guessMsg{place: p, err: err}
	}
}

func searchCmd(name string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		p, err := geo.Search(ctx, nil, geo.DefaultGeocodeEndpoint, name)
		return guessMsg{place: p, err: err, query: name}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.input.Width = max(1, ui.ContentWidth(msg.Width)-6)
		if m.beams != nil {
			m.beams.Resize(max(1, min(msg.Width-4, 100)), ui.BannerHeight())
		}
	case tickMsg:
		m.frame++
		if m.beams != nil {
			m.beams.Update()
		}
		return m, tick()
	case guessMsg:
		m.lookingUp = false
		if msg.err != nil {
			m.weatherNote = msg.err.Error()
		} else {
			candidate := m.w
			candidate.Latitude, candidate.Longitude, candidate.Location = msg.place.Latitude, msg.place.Longitude, msg.place.City
			if strings.IndexFunc(candidate.Location, unicode.IsControl) >= 0 {
				m.weatherNote = i18n.T(m.w.Locale, "weather.invalid")
			} else if _, err := seed.ConfigJSON(candidate.Answers()); err != nil {
				m.weatherNote = err.Error()
			} else {
				m.w = candidate
				if msg.query != "" && strings.TrimSpace(m.weatherQuery) == msg.query {
					m.weatherQuery = ""
					if m.w.Page == ui.PageWeather {
						m.input.SetValue("")
					}
				}
				m.weatherNote = ""
			}
		}
		if m.w.Page == ui.PageWeather {
			m.note = m.weatherNote
		}
	case packageRequestMsg:
		return m, tea.ExecProcess(msg.cmd, func(err error) tea.Msg {
			msg.reply <- err
			return packageFinishedMsg{}
		})
	case progressMsg:
		if m.step == ui.StepInstalling {
			m.tasks = msg.tasks
		}
	case installDoneMsg:
		m.scroll = 0
		m.installRes = msg.res
		m.installErr = msg.err
		if msg.err != nil || anyFailed(msg.res.Tasks) {
			m.step = ui.StepFailed
		} else {
			m.step = ui.StepDone
		}
	case tea.KeyMsg:
		if msg.String() == "pgup" || msg.String() == "pgdown" {
			delta := max(3, m.height/3)
			if msg.String() == "pgup" {
				delta = -delta
			}
			_, body, help := m.content()
			limit := ui.ScrollLimit(m.w.Locale, body, help, m.w.Page, m.step, m.width, m.height, m.beams)
			m.scroll = min(max(0, min(m.scroll, limit)+delta), limit)
			return m, nil
		}
		if m.step == ui.StepInstalling {
			if msg.Type == tea.KeyCtrlC {
				return m, tea.Quit
			}
			return m, nil
		}
		if m.step == ui.StepDone || m.step == ui.StepFailed {
			switch msg.String() {
			case "enter", "q", "ctrl+c":
				return m, tea.Quit
			}
			return m, nil
		}
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "f9":
			m.w = m.w.CycleLocale()
			m.input.Placeholder = i18n.T(m.w.Locale, "weather.place")
			if m.w.Page == ui.PageWallpaper {
				m.input.Placeholder = i18n.T(m.w.Locale, "wallpaper.directory")
			}
		case "esc":
			m.w = m.w.Back()
			m.focusPage()
		case "q":
			if m.w.Page != ui.PageWeather && m.w.Page != ui.PageWallpaper {
				return m, tea.Quit
			}
			return m, m.editInput(msg)
		case "enter":
			switch m.w.Page {
			case ui.PageWallpaper:
				path := strings.TrimSpace(m.input.Value())
				if strings.IndexFunc(path, unicode.IsControl) >= 0 || !(filepath.IsAbs(path) || path == "~" || strings.HasPrefix(path, "~/")) {
					m.note = i18n.T(m.w.Locale, "wallpaper.invalid")
					return m, nil
				}
				m.w.WallpaperDir = path
				m.w = m.w.Next()
				m.focusPage()
			case ui.PageWeather:
				if m.lookingUp {
					return m, nil
				}
				if query := strings.TrimSpace(m.input.Value()); query != "" {
					m.lookingUp = true
					m.note, m.weatherNote = "", ""
					return m, searchCmd(query)
				}
				if m.w.Location == "" {
					m.lookingUp = true
					m.note, m.weatherNote = "", ""
					return m, guessCmd()
				}
				m.w = m.w.Next()
				m.focusPage()
			case ui.PageConfirm:
				m.step = ui.StepInstalling
				m.scroll = 0
				go m.runInstall()
				return m, nil
			default:
				m.w = m.w.Next()
				m.focusPage()
				if m.w.Page == ui.PageWeather && m.w.Location == "" && !m.lookingUp {
					m.lookingUp = true
					return m, guessCmd()
				}
			}
		case "left", "right":
			if m.w.Page == ui.PageTheme {
				m.w = m.w.CycleMode()
			} else if m.w.Page == ui.PageConflicts {
				m.w = m.w.CycleChoice(msg.String() == "right")
			} else {
				return m, m.editInput(msg)
			}
		case "up", "down":
			m.cycle(msg.String() == "down")
		case " ":
			if m.w.Page == ui.PagePlugins {
				m.w = m.w.TogglePlugin()
			} else {
				return m, m.editInput(msg)
			}
		default:
			return m, m.editInput(msg)
		}
	}

	return m, nil
}

func (m *model) focusPage() {
	m.scroll, m.note = 0, ""
	m.input.Blur()
	if m.w.Page == ui.PageWallpaper {
		m.input.SetValue(m.w.WallpaperDir)
		m.input.Placeholder = i18n.T(m.w.Locale, "wallpaper.directory")
		m.input.Focus()
	}
	if m.w.Page == ui.PageWeather {
		m.input.SetValue(m.weatherQuery)
		m.note = m.weatherNote
		m.input.Placeholder = i18n.T(m.w.Locale, "weather.place")
		m.input.Focus()
	}
}

func (m *model) editInput(msg tea.KeyMsg) tea.Cmd {
	if m.w.Page != ui.PageWallpaper && m.w.Page != ui.PageWeather {
		return nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	if m.w.Page == ui.PageWallpaper {
		m.w.WallpaperDir = m.input.Value()
		m.note = ""
	} else {
		m.weatherQuery = m.input.Value()
	}
	return cmd
}

func (m *model) cycle(down bool) {
	step := 1
	if !down {
		step = -1
	}
	switch m.w.Page {
	case ui.PageTheme:
		m.w.Preset = cycle([]string{"standard", "compact", "expressive"}, m.w.Preset, step)
	case ui.PagePlugins:
		m.w = m.w.MovePlugin(step)
	case ui.PageConflicts:
		m.w = m.w.MoveConflict(boolToDelta(down))
	}
	if m.w.Page == ui.PagePlugins || m.w.Page == ui.PageConflicts {
		m.keepChoiceVisible()
	}
}

// Keep the focused control visible using rendered rows, including wrapping.
func (m *model) keepChoiceVisible() {
	_, body, help := m.content()
	lines := strings.Split(ansi.Wrap(body, ui.ContentWidth(m.width), ""), "\n")
	limit := ui.ScrollLimit(m.w.Locale, body, help, m.w.Page, m.step, m.width, m.height, m.beams)
	rows := len(lines) - limit
	for i, line := range lines {
		if !strings.Contains(ansi.Strip(line), "> ") {
			continue
		}
		end := i + 1
		if m.w.Page == ui.PageConflicts {
			end = len(lines) - 1 // Exclude the control's bottom border.
			if next := m.w.ConflictRow + 1; next < len(m.w.Findings) {
				for j := i + 1; j < len(lines); j++ {
					if strings.Contains(ansi.Strip(lines[j]), m.w.Findings[next].Name) {
						end = j
						break
					}
				}
			}
		}
		end = min(end, i+rows)
		m.scroll = min(limit, max(0, max(min(m.scroll, i), end-rows)))
		break
	}
}

func boolToDelta(down bool) int {
	if down {
		return 1
	}
	return -1
}

func cycle(values []string, current string, step int) string {
	for i, v := range values {
		if v == current {
			return values[(i+step+len(values))%len(values)]
		}
	}
	return values[0]
}

// runInstall executes the injected installer in the background, teeing every
// task snapshot to the log file and the tea program.
func (m model) runInstall() {
	if m.installer == nil {
		m.notify(installDoneMsg{err: errors.New("installer is not wired up")})
		return
	}
	defer func() {
		if r := recover(); r != nil {
			m.notify(installDoneMsg{err: fmt.Errorf("installer panicked: %v", r)})
		}
	}()
	if err := os.MkdirAll(filepath.Dir(m.logPath), 0o700); err != nil {
		m.notify(installDoneMsg{err: err})
		return
	}
	logFile, err := os.OpenFile(m.logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		m.notify(installDoneMsg{err: err})
		return
	}
	defer logFile.Close()
	progress := func(tasks []install.Task) {
		for _, task := range tasks {
			status := string(task.Status)
			if status == "" {
				status = "pending"
			}
			line := task.Name + ": " + status
			if task.Reason != "" {
				line += " (" + task.Reason + ")"
			}
			fmt.Fprintln(logFile, line)
		}
		m.notify(progressMsg{tasks: append([]install.Task(nil), tasks...)})
	}
	res, err := m.installer(m.w.Answers(), progress)
	if err != nil {
		fmt.Fprintf(logFile, "error: %v\n", err)
	}
	m.notify(installDoneMsg{res: res, err: err})
}

func (m model) notify(msg tea.Msg) {
	if m.send != nil {
		m.send(msg)
	}
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// taskLines renders one line per task: spinner while pending, then a status
// glyph with the reason when there is one.
func taskLines(tasks []install.Task, frame int) string {
	lines := make([]string, 0, len(tasks))
	for _, task := range tasks {
		switch task.Status {
		case install.Done:
			lines = append(lines, "✓ "+task.Name)
		case install.Skipped:
			lines = append(lines, "- "+task.Name+reason(task.Reason))
		case install.Failed:
			lines = append(lines, "✗ "+task.Name+reason(task.Reason))
		default:
			lines = append(lines, spinnerFrames[frame%len(spinnerFrames)]+" "+task.Name)
		}
	}
	return strings.Join(lines, "\n")
}

func reason(r string) string {
	if r == "" {
		return ""
	}
	return " (" + r + ")"
}

func (m model) content() (string, string, string) {
	title, body, help := m.w.Title(), m.w.BodyWidth(max(1, ui.ContentWidth(m.width))), m.w.Help()
	switch m.step {
	case ui.StepInstalling:
		title = i18n.T(m.w.Locale, "install.title")
		body = i18n.T(m.w.Locale, "install.blurb") + "\n\n" + taskLines(m.tasks, m.frame)
		help = i18n.T(m.w.Locale, "help.install")
	case ui.StepDone, ui.StepFailed:
		prefix := "done"
		if m.step == ui.StepFailed {
			prefix = "failed"
		}
		title, help = i18n.T(m.w.Locale, prefix+".title"), i18n.T(m.w.Locale, "help."+prefix)
		body = i18n.T(m.w.Locale, prefix+".blurb") + "\n\n" + taskLines(m.installRes.Tasks, 0)
		if m.installErr != nil {
			body += "\n\n" + m.installErr.Error()
		}
		if m.installRes.SessionWarning != "" {
			body += "\n\n" + m.installRes.SessionWarning
		}
		body += "\n\n" + i18n.T(m.w.Locale, "install.log") + ": " + m.logPath
		if len(m.installRes.Warnings) > 0 {
			body += "\n\n" + strings.Join(m.installRes.Warnings, "\n")
		}
	default:
		if m.w.Page == ui.PageWallpaper {
			body = ui.Control(i18n.T(m.w.Locale, "wallpaper.directory"), m.input.View(), ui.ContentWidth(m.width), true)
		}
		if m.w.Page == ui.PageWeather {
			body += "\n" + ui.Control(i18n.T(m.w.Locale, "weather.city"), m.input.View(), ui.ContentWidth(m.width), true)
			if m.lookingUp {
				body += "\n" + spinnerFrames[m.frame%len(spinnerFrames)] + " " + i18n.T(m.w.Locale, "weather.searching")
			}
		}
		if (m.w.Page == ui.PageWeather || m.w.Page == ui.PageWallpaper) && m.note != "" {
			body += "\n! " + m.note
		}
	}
	return title, body, help
}

func (m model) View() string {
	title, body, help := m.content()
	return ui.ViewWithHelp(m.w.Locale, title, body, help, m.w.Page, m.step, m.width, m.height, m.beams, m.scroll)
}

// newInstallProgram wires send and the installer before the program copies
// the model: the running copy must not be left with nil seams (#35).
func newInstallProgram(m model, installer func(seed.Answers, func([]install.Task)) (install.Result, error), opts ...tea.ProgramOption) *tea.Program {
	var prog *tea.Program
	m.send = func(msg tea.Msg) { prog.Send(msg) }
	m.installer = installer
	prog = tea.NewProgram(m, opts...)
	return prog
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "uninstall" {
		os.Exit(runUninstall(os.Args[2:], os.Stdin, os.Stdout, mustHome()))
	}
	o, err := parseFlags(os.Args[1:], os.Stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	loc := i18n.Match(firstNonEmpty(o.Lang, os.Getenv("LANG")))
	osr, _ := os.ReadFile("/etc/os-release")
	wayland, niriSocket := os.Getenv("WAYLAND_DISPLAY"), os.Getenv("NIRI_SOCKET")
	if err := preflight.Check(preflight.Env{
		EUID:           os.Geteuid(),
		OSRelease:      osr,
		WaylandDisplay: wayland,
		NiriSocket:     niriSocket,
		Locale:         loc,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	inNiri := liveNiriSession(wayland, niriSocket)
	p, err := pin.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ctx := context.Background()
	findings, _ := conflict.Detect(conflict.Env{NiriConfig: niriConfigPath(home)})
	choices := conflict.Defaults(findings, o.KeepConflicts, o.Handover == "all")

	if o.Yes {
		a, err := answersFor(ctx, o, p.Recommended)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		opts := installOptions(home, p, a, true, loc, osr, runPackage)
		opts.InNiriSession = inNiri
		opts.Findings = findings
		opts.Conflicts = choices
		res, err := install.Run(ctx, opts)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		printTasks(os.Stdout, res)
		if anyFailed(res.Tasks) {
			os.Exit(1)
		}
		return
	}

	plainNiri := inNiri && !graphicalSessionActive()
	var prog *tea.Program
	installer := func(a seed.Answers, progress func([]install.Task)) (install.Result, error) {
		opts := installOptions(home, p, a, false, loc, osr, func(cmd *exec.Cmd) error {
			reply := make(chan error, 1)
			prog.Send(packageRequestMsg{cmd: cmd, reply: reply})
			return <-reply
		})

		opts.InNiriSession = inNiri
		opts.Progress = progress
		opts.Findings = findings
		opts.Conflicts = choices
		return install.Run(ctx, opts)
	}
	m := newModel(loc, p.Recommended, plainNiri)
	m.w.Findings = findings
	m.w.Choices = choices
	for _, c := range p.Components {
		if !c.Disabled {
			m.w.Suite = append(m.w.Suite, c.ID)
		}
	}
	if _, err := os.Stat(filepath.Join(configHome(home), "sysc-shell", "config.json")); err == nil {
		m.w.ExistingConfig = true
	}
	m.w.PackageAdviceKey = "confirm.system"
	m.w.PackageManager, _ = gslapperManager(osr)
	if _, err := exec.LookPath("gslapper"); err == nil {
		m.w.PackageAdviceKey = "confirm.system.present"
	} else {
		if m.w.PackageManager == "" {
			m.w.PackageAdviceKey = "confirm.system.missing"
		}
		if m.w.PackageManager == "apt-get" || m.w.PackageManager == "dnf" {
			if _, err := gslapperAsset(osr, p, runtime.GOARCH); err != nil {
				m.w.PackageAdviceKey = "confirm.system.missing"
			}
		}
	}

	prog = newInstallProgram(m, installer, tea.WithAltScreen())
	final, err := prog.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fm, ok := final.(model)
	if !ok {
		return
	}
	if fm.installErr != nil || anyFailed(fm.installRes.Tasks) {
		if fm.installErr != nil {
			fmt.Fprintln(os.Stderr, fm.installErr)
		}
		os.Exit(1)
	}
}

func printTasks(out io.Writer, res install.Result) {
	for _, task := range res.Tasks {
		line := fmt.Sprintf("%s: %s", task.Name, task.Status)
		if task.Reason != "" {
			line += " (" + task.Reason + ")"
		}
		fmt.Fprintln(out, line)
	}
	// The stamp is written only after enable. Started stays false when the
	// units were enabled and not started (SSH, TTY, or plain niri).
	if res.Stamp.Release != "" && !res.Stamp.Started {
		fmt.Fprintln(out, "units: enabled but not started")
	}
	if res.SessionWarning != "" {
		fmt.Fprintln(out, res.SessionWarning)
	}
	for _, w := range res.Warnings {
		fmt.Fprintln(out, w)
	}
}

// configHome follows the same XDG rule as the installer.
func configHome(home string) string {
	if base := os.Getenv("XDG_CONFIG_HOME"); filepath.IsAbs(base) {
		return base
	}
	return filepath.Join(home, ".config")
}

// niriConfigPath is the compositor config conflicts are detected against.
func niriConfigPath(home string) string { return filepath.Join(configHome(home), "niri", "config.kdl") }

func graphicalSessionActive() bool {
	return units.GraphicalSessionActive(func(args ...string) error {
		return exec.Command("systemctl", append([]string{"--user"}, args...)...).Run()
	})
}

// stateHome mirrors the installer's XDG state rule so the TUI log lands in
// the same tree as the stamp and staging dir.
func stateHome() string {
	if v := os.Getenv("XDG_STATE_HOME"); filepath.IsAbs(v) {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return os.TempDir()
	}
	return filepath.Join(home, ".local", "state")
}

func mustHome() string {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return home
}

// liveNiriSession reports a compositor the user is inside right now.
// A missing WAYLAND_DISPLAY (SSH, TTY) is not a live session.
func liveNiriSession(wayland, socket string) bool {
	return wayland != "" && socket != ""
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
