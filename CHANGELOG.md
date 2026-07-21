# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.3.0] - 2026-07-21

### Added

- Guest control: run a command inside a running Linux guest and copy a file out
  of it, from a new panel. Credentials are used once per session (never stored)
  and passed to VBoxManage via a temporary `--passwordfile`, never on a command
  line. (#30)
- Save state (suspend): freeze a running VM's memory to disk with a single
  action; resume exactly where it left off with the existing state-aware Start.
  (#33)
- USB passthrough: list the host's USB devices and attach/detach one to a
  running VM, with the Oracle Extension Pack and USB-controller prerequisites
  detected and surfaced instead of failing obscurely. (#35)
- NIC link state: connect or disconnect a NIC's virtual network cable per
  adapter — live on a running VM, in config on a stopped one — to simulate an
  outage without changing the attachment mode. (#37)

### Changed

- VBoxManage calls are now serialized per VM (with a global concurrency cap),
  removing the session-lock contention that could wedge VBoxSVC when several
  operations or reads overlapped on the same VM. (#23)

## [0.2.0] - 2026-07-20

### Added

- In-app update notification: the UI shows a dismissible banner when a newer
  release exists on GitHub, linking to the download. The check is best-effort,
  cached, local-first (fails silently offline), and can be disabled. (#24)
- Multi-NIC NAT port forwarding: add and remove per-NIC host→guest forwarding
  rules from the Network panel, with a loopback-safe default host IP and
  cross-VM host-port collision guards. (#25)
- Clone a VM: duplicate a stopped VM as a full or linked clone (linked clones
  are based on the source's current snapshot), run as an async job. (#26)
- Export a VM to OVA: export a stopped VM to a portable `.ova` appliance as an
  async job, with strict destination-path validation. (#27)

## [0.1.2] - 2026-07-20

### Fixed

- VM operation failures now surface an actionable, sanitized reason instead of a
  generic message: known VBoxManage errors (session lock, missing hardware
  virtualization, low host memory) map to clear guidance, while the real cause
  (exit code and stderr) is recorded in the operation log. (#21)
- VM start is now state-aware: it is idempotent when the VM is already running,
  resumes a paused VM instead of failing, never force-powers-off (preserving a
  saved state), and retries transient VirtualBox session-lock contention. (#21)

## [0.1.1] - 2026-07-13

### Added

- Repository foundation: `.gitignore`/`.gitattributes` hardening, cross-platform
  `npm run build` (web UI → embedded agent), CI workflow, Dependabot, issue/PR
  templates, `SECURITY.md`, `CONTRIBUTING.md`, and `CODE_OF_CONDUCT.md`.
- Software version surfaced in the sidebar and exposed via `GET /health`
  (single source of truth in `internal/version`).

### Fixed

- Launcher now compares the running agent's `/health` version against its own:
  a stale agent left over from a previous install is stopped and replaced, so
  launching after an upgrade never opens the old embedded web UI.
- Installer closes running TabVM processes (`tabvm-agent.exe`, `TabVM.exe`)
  before upgrading, so files are never replaced under a live agent.
- Scoop manifest stops a running agent before install/update and on uninstall
  (`pre_install` / `pre_uninstall`), so package-manager upgrades take effect
  immediately.

## [0.1.0] - 2026-07-11

### Added

- Local Go agent bound to `127.0.0.1` with a session-token-protected API,
  wrapping `VBoxManage` for VM lifecycle, console (VRDE/RDP), telemetry, shared
  folders, clipboard, Guest Additions, snapshots, networking, hardware (vCPU /
  memory), and disk storage (resize / add / detach / delete).
- React + Vite web UI: machines dashboard, browser console, Files panel,
  drag-and-drop transfer, create-VM wizard, in-app interactive manual, theme /
  accent personalization, and EN/ES localization.
- Windows packaging: portable ZIP and Inno Setup installer with the web UI
  embedded via `go:embed`.

[Unreleased]: https://github.com/carauz1905/TabVM/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/carauz1905/TabVM/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/carauz1905/TabVM/compare/v0.1.2...v0.2.0
[0.1.2]: https://github.com/carauz1905/TabVM/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/carauz1905/TabVM/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/carauz1905/TabVM/releases/tag/v0.1.0
