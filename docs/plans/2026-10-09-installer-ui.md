# Installer UI redesign

Goal: restore the bordered, keyboard guided installer language used by
sysc-greet and Moonbit, with a pure black canvas and help on every page.

Use the existing Bubble Tea, Bubbles text input, and Lip Gloss dependencies.
Keep user files and systemd user services unprivileged. The gSlapper package
step may elevate through the distro's package manager. Arch uses yay/paru;
Debian/Ubuntu and Fedora use checksum-pinned release packages with apt-get
and dnf. The suite's Arch-only preflight stays in place until the remaining
components have been validated on other distros.

1. Replace the chrome with an adaptive black canvas, page progress, bordered
   controls, contextual advice, explicit actions, and a scrollable body.
   Keep the large SYSC banner on tall terminals; prioritize forms at 80×24.
2. Render presets as radio choices and plugins as independent checkboxes.
   Make the wallpaper path editable and validate it before advancing.
   Show weather lookup feedback, retain editable values when navigating back,
   and keep long content and focused choices reachable.
3. Show the current suite and its user/system footprint in review. Run package
   commands with Bubble Tea's terminal handoff so the helper can request sudo.
   Preserve package failure reporting, existing configuration, and uninstall.
4. Update localized copy and layout checks. Leave a focused runnable regression
   check for selection, input, scrolling, and terminal handoff. Build and run
   the affected checks, then inspect representative screens locally.

UI/UX Pro Max's OLED style and keyboard focus advice apply. Its web landing
page and CSS/font recommendations do not fit this terminal application.
Use #000000 everywhere, #ffffff for active borders/text, #a3a3a3 for advice,
and visible textual markers for selected, skipped, failed, and pending states.
