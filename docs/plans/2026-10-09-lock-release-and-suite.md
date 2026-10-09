# sysc-lock initial release and suite installation

**Goal:** Release sysc-lock v0.1.0 and install/enable it by default through sysc.

**Architecture:** Use the existing verified asset download, unit installation, startup ordering and uninstall stamp. Resolve unit templates from the component's declared unit, since sysc-lock uses sysc-lock-session.service. Seed session.locker=sysc-lock only in new shell configs; preserve existing files. Stop only units being installed or stamped as owned; attempt every selected stop even if one fails. No new dependencies or service abstraction.

**Release:** Build Linux amd64 on Ubuntu 22.04 with PAM/EGL/GLES dependencies and CGO_CFLAGS=-D_GNU_SOURCE, needed by the PAM binding with older glibc headers. Publish sysc-lock, the session unit and SHA256SUMS. Set the compiled version to 0.1.0. Support workflow_dispatch for artifact proof before the tag. Check version and description without acquiring a lock.

**Implementation sequence:**

1. Add a failing version/release-contract check in sysc-lock, change cmd/sysc-lock/main.go and add .github/workflows/release.yml. Run focused command/installer tests, vet and build; locally review and merge. Dispatch the release workflow without publishing, verify downloaded binary/metadata, then tag the tested merge as v0.1.0 and verify public assets.
2. Add installer checks for enabled lock pin, declared session unit, fresh locker seed, service enable/start and uninstall. Run them to confirm failures. Remove the obsolete lock prohibition from internal/pin/pin.go, resolve units by c.Unit in internal/install/install.go, embed the authoritative session unit and extend ordering in internal/units/units.go. Seed session.locker in internal/seed/seed.go. Adapt existing rejection/order checks to the new contract.
3. Put the verified public URL and SHA256 into internal/pin/pin.json; update README.md. Run bounded Go tests/vet/build and an install/uninstall check in a temporary home with fake systemctl. Download and verify the real release artifact without touching the user's services. Review locally, pass CI and merge to origin/main.

**Checks:** GOMAXPROCS=2, go test -p 1 -count=1 on affected packages; go vet -p 1 ./...; go build -p 1. Full race checks run in CI. Preserve current checkout changes; work in /home/nomadx/worktrees/sysc-install-lock and /home/nomadx/worktrees/sysc-lock-initial-release. Track execution in beads sysc-greet-dev-63.

**Scope:** The current shell release pin stays unchanged while its media/weather work proceeds. No suite release tag or greeter release is part of this pass. Enable the lock owner by default; do not acquire a lock during installation.
