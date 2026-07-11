# Architecture

TabVM turns local VirtualBox machines into a browser-managed workspace. The
browser is only the user interface; a trusted local Go agent performs every
privileged operation.

```text
Browser
  │
  ▼
TabVM Web UI (React + Vite, embedded in the agent)
  │  HTTP on 127.0.0.1, session-token protected
  ▼
TabVM Local Agent (Go)
  │  explicit VBoxManage argument arrays (no shell concatenation)
  ▼
VBoxManage / VirtualBox
  │
  ▼
VM headless + VRDE/RDP console
```

## Components

### Local agent (`apps/desktop-agent`)

The agent owns all privileged host operations and exposes a narrow API bound to
`127.0.0.1`:

- Discover VirtualBox and `VBoxManage`.
- List VMs with best-effort real state; start / stop (ACPI) / reset / pause.
- Prepare the browser console (VRDE/RDP on a deterministic loopback port).
- Manage snapshots, networking, hardware (vCPU / memory), and disk storage.
- Transfer files, shared folders, clipboard, and Guest Additions via
  `guestcontrol` / `guestproperty`.
- Persist non-secret local state in SQLite.
- Serve the web UI from an embedded `go:embed` bundle.

### Web UI (`apps/web-ui`)

React + TypeScript + Vite. It is built independently and embedded into the agent
binary at compile time. It provides the machines dashboard, the browser console,
file transfer, the create-VM wizard, the in-app manual, and EN/ES localization.

## Why native Windows first

Most classroom support risk comes from installation and environment differences.
A native Windows agent keeps the support burden low and packaging simple,
compared with a WSL/Docker stack (useful for prototypes) or a centralized
Proxmox/KVM lab (a different product).

## Recommended stack

| Area | Choice | Reason |
|------|--------|--------|
| Agent | Go | Lightweight native binary, easy distribution. |
| Web UI | React + TypeScript + Vite | Fast local dev, strong component model. |
| Local state | SQLite (`modernc.org/sqlite`, pure Go) | Simple local persistence, no external DB. |
| VM control | `VBoxManage` | Stable, scriptable, avoids deeper VirtualBox APIs. |
| Console | VirtualBox VRDE/RDP | Console access independent of the guest OS. |
| Installer | Inno Setup | Native Windows installer, portable ZIP alternative. |
| Build/release | GitHub Actions + GitHub Releases | Repeatable CI/CD and open distribution. |

## Console access

The agent enables VirtualBox VRDE/RDP on `127.0.0.1` with a deterministic port
derived from the VM ID, connects to that RDP server, and streams the guest
framebuffer to the browser over a WebSocket. The web UI paints it on a
`<canvas>` (`ScreenConsole`), with a periodic `InvalidateAndUpdate` keepalive
and letterboxed scaling so large guest resolutions fit without cropping. The
console works independently of the guest operating system, which is useful for
installation and recovery.

> [!NOTE]
> VRDE/RDP may require the VirtualBox Extension Pack and a licensing review in
> institutional environments.

## Security model

See [`../SECURITY.md`](../SECURITY.md). In short: the agent is the trusted
boundary, binds to loopback only, requires a session token, exposes no arbitrary
shell execution, validates every operation, and keeps guest credentials out of
argv, logs, and the database.
