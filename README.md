# TabVM

**Every VM. One tab.**

TabVM turns your local VirtualBox machines into browser tabs. Install nothing but
VirtualBox — everything else runs on your own computer, from one window on
`127.0.0.1`. It is built for students and lab users who want to operate a VM
without the VirtualBox window competing with the rest of their desktop.

> [!NOTE]
> TabVM controls your **local** VirtualBox VMs. It is not a cloud lab, and it
> never sends your machines anywhere — the agent binds to loopback only.

## Features

- **Machines dashboard** — start, stop (ACPI), reset, pause, and delete VMs with
  real-time state.
- **Browser console** — view and drive the guest screen in a tab via VRDE/RDP,
  with a live-refresh keepalive and letterboxed fit.
- **Files & transfer** — host folder picker, auto-named shared folders, and
  host→guest drag-and-drop (shared folder or `guestcontrol`).
- **Snapshots** — take, restore, and delete snapshots from the focus view.
- **Hardware & storage** — adjust vCPU / memory; resize, add, detach, and delete
  disks.
- **Networking** — switch per-NIC mode (NAT / bridged / host-only), live or offline.
- **Guest Additions** — detect, install, and update GA per VM.
- **Create VM** — import a `.ova` appliance or install a fresh Ubuntu/Debian
  guest unattended, with Guest Additions baked in.
- **Guided manual** — an in-app interactive manual with animated demos of every
  control.
- **Made yours** — light/dark themes, accent colors, and EN/ES localization,
  persisted locally.

## Quick start

> [!IMPORTANT]
> Requires [Oracle VirtualBox](https://www.virtualbox.org/) with `VBoxManage`
> available on the host.

**Users** — download the latest installer or portable ZIP from
[Releases](../../releases), run it, and your browser opens the dashboard at
`127.0.0.1`. No login, no cloud.

**Developers**:

```bash
npm ci        # install workspace dependencies
npm run build # build the web UI, embed it, and build the agent
```

Then run the agent (`apps/desktop-agent/tabvm-agent.exe`) and open the printed
`127.0.0.1` URL. See [`docs/running-locally.md`](docs/running-locally.md) for the
development workflow.

> [!IMPORTANT]
> The embedded web UI (`apps/desktop-agent/internal/webui/dist`) is **generated**
> and git-ignored. Run `npm run build` before `go build`/`go test` on a fresh
> clone.

## How it works

The browser is only the UI. A trusted local Go agent performs every privileged
operation through `VBoxManage`, bound to `127.0.0.1` and protected by a session
token.

```text
Browser → TabVM Web UI → TabVM Agent (Go) → VBoxManage → VirtualBox → VM + VRDE/RDP
```

See [`docs/architecture.md`](docs/architecture.md) for the full picture and
[`SECURITY.md`](SECURITY.md) for the security model.

## Project layout

```text
apps/desktop-agent/   Go agent: VBoxManage wrapper, HTTP API, embedded UI
apps/web-ui/          React + TypeScript + Vite front-end
packages/shared/      Shared code
scripts/              Build, release, and dev helpers
docs/                 Architecture, security, and workflow docs
installer/            Inno Setup script
```

## Roadmap

- Serial-console terminal in a tab for Linux guests (firewall/policy-immune).
- In-app update awareness and one-click updates from signed GitHub Releases.
- Resizable split panels and multiple simultaneous sessions.
- Lab templates.

## Documentation

- [Architecture](docs/architecture.md)
- [Running locally](docs/running-locally.md)
- [MVP definition](docs/mvp.md)
- [Contributing](CONTRIBUTING.md) · [Security](SECURITY.md) · [Code of Conduct](CODE_OF_CONDUCT.md)
