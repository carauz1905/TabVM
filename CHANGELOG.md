# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.4.0] - 2026-07-24

### Added

- In-manual search: filter the manual's table of contents and sections by any
  text in the active language, case- and diacritic-insensitively, with a
  no-results state and a clear button. (#58)
- The Agent view shows the latest available release next to the runtime info,
  reusing the update check. (#52)
- USB device states are translated and carry explanatory tooltips (what Busy,
  Available, Captured and Unavailable mean). (#52)
- A stop request the guest ignores now surfaces: after a grace period the row
  shows a dismissible notice and the force power-off action becomes visible.
  (#46)
- "New tab" and "terminal" have a visible secondary emphasis in the running
  row — outlined at rest, filling with the accent on hover — so the most
  powerful console features no longer look like minor actions. (#54)

### Changed

- The first VM is focused automatically on load (preferring a running one), so
  the dashboard never opens onto an empty focus area. (#52)
- The Install/Update Guest Additions call-to-action moved from the VM row to a
  notice card in the focus section, keeping rows lean. (#52)
- The telemetry rail shows the configured vCPU count and memory for stopped
  VMs instead of dashes. (#52)
- The activity log is readable: internal action codes render as localized
  labels, success uses a distinct green dot, and timestamps follow the active
  language's date format. (#48)
- The Spanish UI and manual use one consistent formal register, and native
  form controls follow the in-app theme instead of the OS preference. (#50,
  #46)

### Fixed

- Focusing a VM without snapshots no longer produces a 502 and an error log
  entry on every focus. (#44)
- USB attach is disabled with an explanation when the VM has no USB
  controller, instead of failing after the click. (#46)
- Guest-control credential fields no longer render as black boxes in the light
  theme when the OS prefers dark. (#46)
- The serial terminal is no longer offered on stopped VMs. (#46)
- Hardware and Storage panels re-gate immediately when the focused VM starts
  or stops, instead of keeping a stale editable state. (#52)
- Port-forwarding fields have visible labels instead of truncated
  placeholders; MAC addresses render with separators; the Guest Additions
  call-to-action no longer pulses forever. (#50)

## [0.3.2] - 2026-07-22

### Fixed

- Running VM row: the action bar no longer overlaps the machine name and UUID.
  On a running VM the button set could collapse the name/UUID column to zero
  width and render on top of it, wrapping the UUID across several lines; the
  column now keeps a minimum width with the name and UUID on one line, and the
  action bar wraps to a second line instead of overflowing. (#42)

## [0.3.1] - 2026-07-22

### Fixed

- Starting a VM now retries automatically on the transient
  `VERR_UNRESOLVED_ERROR` host-platform failure that VirtualBox intermittently
  returns under an active Windows hypervisor (Hyper-V/VBS), so a self-clearing
  error no longer surfaces to the user. A genuinely persistent failure still
  surfaces after the bounded retries. (#40)

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

### Fixed

- The global VBoxManage concurrency cap no longer lets several long operations
  starve fast reads: the wait for a free slot is now bounded by the command's
  own timeout, so a status or VM-list refresh fails fast instead of freezing the
  dashboard while four multi-minute operations run. (#39)
- Mounting an ISO no longer fails when a VM has a floppy controller enumerated
  before its DVD drive: optical-drive detection skips the floppy bus and finds
  the real optical-capable controller. (#39)
- The USB panel surfaces a load error instead of showing a misleading "no USB
  devices" state when host USB enumeration fails. (#39)

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

[Unreleased]: https://github.com/carauz1905/TabVM/compare/v0.4.0...HEAD
[0.4.0]: https://github.com/carauz1905/TabVM/compare/v0.3.2...v0.4.0
[0.3.2]: https://github.com/carauz1905/TabVM/compare/v0.3.1...v0.3.2
[0.3.1]: https://github.com/carauz1905/TabVM/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/carauz1905/TabVM/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/carauz1905/TabVM/compare/v0.1.2...v0.2.0
[0.1.2]: https://github.com/carauz1905/TabVM/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/carauz1905/TabVM/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/carauz1905/TabVM/releases/tag/v0.1.0
