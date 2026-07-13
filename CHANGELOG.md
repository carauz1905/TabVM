# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/carauz1905/TabVM/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/carauz1905/TabVM/releases/tag/v0.1.0
