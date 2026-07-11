# Running TabVM Locally on Windows

This document explains how to run TabVM on a Windows development machine.

## Prerequisites

| Tool | Purpose | Download |
|------|---------|----------|
| Go 1.26 or later | Build and run the desktop agent | https://go.dev/dl/ |
| Node.js ^20.19.0 or >=22.12.0 and npm | Build and run the web UI | https://nodejs.org/ |
| VirtualBox | VM discovery and control | https://www.virtualbox.org/ |

You can verify prerequisites with the included helper:

```powershell
.\scripts\check-prereqs.ps1
```

The script checks for Go, Node.js, npm, and the default VirtualBox installation
paths.

## Repository layout

```text
TabVM/
├─ apps/
│  ├─ desktop-agent/   Go local agent (VBoxManage wrapper, HTTP API, embedded UI)
│  └─ web-ui/          React + TypeScript + Vite UI
├─ packages/
│  └─ shared/          API contract notes
├─ scripts/            Build, release, and dev helpers
└─ docs/               Project documentation
```

## Session token

The agent treats `/api/*` as a privileged boundary. Every request to `/api/*`
must include the `X-TabVM-Session-Token` header. `/health` remains
unauthenticated and non-sensitive.

`scripts/dev-start.ps1` generates a random per-run development token and exports
it to both processes before launching:

- Agent: `TABVM_AGENT_SESSION_TOKEN` (or the legacy `TabVM__Agent__SessionToken`).
- Web UI: `VITE_TABVM_SESSION_TOKEN`.

For a manual agent run or a production build, set the token consistently. If
`TABVM_AGENT_SESSION_TOKEN` is not configured and the agent runs in Development
mode, it generates a one-time random token and logs that a temporary development
token is in use (the token value itself is not logged). That generated token is
only suitable for local development.

No real token is committed to source control. Copy `apps/web-ui/.env.example` to
`apps/web-ui/.env.local` and fill in a real token when running the UI manually.

### Local state database

The agent persists minimal local state in a SQLite database. By default it is
created at `%LOCALAPPDATA%\TabVM\tabvm.db`. Override the location with:

```powershell
# Custom data directory; the database will be <DataDir>\tabvm.db
$env:TABVM_DATA_DIR = "C:\Path\To\TabVM\Data"

# Or point to an explicit database file
$env:TABVM_DB_PATH = "C:\Path\To\tabvm.db"
```

`TABVM_DB_PATH` takes precedence over `TABVM_DATA_DIR`. Paths containing `..` are
rejected. If the database cannot be opened or initialized, the agent exits during
startup.

What is stored:

- VRDE console port assignments per VM (`vm_console_ports`), so ports remain
  stable across restarts.
- Non-secret application settings (`app_settings`), when persisted by a
  management flow. Environment variables still take precedence.
- A bounded operation log (`operation_log`) for VM lifecycle and console actions.

What is NOT stored: session tokens, passwords, or other secrets; full VBoxManage
paths or host diagnostics; clipboard content or VM display data.

## Start both processes

Run the development launcher from the repository root:

```powershell
.\scripts\dev-start.ps1
```

It runs `check-prereqs.ps1`, installs frontend dependencies, generates a per-run
token, starts the agent on `http://127.0.0.1:5230` (`go run .`), verifies
readiness through the authenticated discovery endpoint, and starts the Vite dev
server on `http://localhost:5173` (which proxies `/api` and `/health` to the
agent). Then open `http://localhost:5173`.

## Start the agent manually

```powershell
cd apps\desktop-agent
$env:TABVM_AGENT_SESSION_TOKEN = "your-dev-token"
go run .
```

The agent binds to localhost only. The default address is `127.0.0.1`, overridden
with `TABVM_AGENT_BIND_ADDRESS` and `TABVM_AGENT_BIND_PORT`. For security, the
bind address must be a loopback address (`127.0.0.1`, `::1`, or `localhost`);
LAN addresses like `0.0.0.0` are rejected at startup.

Core endpoints (all `/api/*` require `X-TabVM-Session-Token`):

- `GET /health` — liveness and software `version` (no token required).
- `GET /api/vbox/discovery` — VirtualBox / VBoxManage discovery.
- `GET /api/local-state/status` — SQLite configured/available status and schema.
- `GET /api/vms` — registered VMs with best-effort real state.
- `GET /api/vms/{id}/status` — detailed VM state.
- `POST /api/vms/{id}/start` — `VBoxManage startvm {id} --type headless`.
- `POST /api/vms/{id}/stop` — `VBoxManage controlvm {id} acpipowerbutton`.
- `POST /api/vms/{id}/reset` — `VBoxManage controlvm {id} reset`.
- `GET /api/vms/{id}/console` — console status and targets.
- `POST /api/vms/{id}/console/prepare` — enable VRDE/RDP on a deterministic
  loopback port and produce an `rdp` target (`source=virtualbox-vrde`).
- `POST /api/vms/{id}/console/disable` — disable VRDE/RDP.

Further VM operations (telemetry, snapshots, networking, hardware, storage,
shared folders, clipboard, Guest Additions, file transfer, create/import) are
dispatched under `/api/vms/…`; see `apps/desktop-agent/internal/server` for the
current set.

Only UUID-shaped VM identifiers are accepted. Concurrent lifecycle operations on
the same VM are rejected with `409 Conflict`. Failures are logged server-side
with full `VBoxManage` stderr but returned to the UI as sanitized messages.

## Start the web UI manually

```powershell
cd apps\web-ui
copy .env.example .env.local
# Edit .env.local and set VITE_TABVM_SESSION_TOKEN to match the agent token
npm install
npm run dev
```

The dev server proxies `/api` and `/health` to the agent, avoiding CORS issues.

## Build for production

From the repository root, one command builds the web UI, embeds it into the
agent, and builds the binary:

```powershell
npm ci
npm run build
```

> [!IMPORTANT]
> The embedded UI (`apps/desktop-agent/internal/webui/dist`) is generated and
> git-ignored, so `npm run build` must run before `go build`/`go test` on a
> fresh clone.

## Testing

```powershell
npm test              # web UI (vitest) + agent (go test)
npm run webui:test    # web UI only
npm run agent:test    # agent only
```

Focused tests (`it.only` / `describe.only`) are rejected by the Vitest
configuration and will fail the run.

## Notes

- The agent does not expose arbitrary shell execution.
- Stop uses `acpipowerbutton` for a graceful ACPI shutdown request. If a VM does
  not respond to ACPI, use reset or shut it down inside the guest.
- The browser console is a direct stream: the agent enables VirtualBox VRDE/RDP
  on `127.0.0.1` (deterministic port in `5000-5999` derived from the VM ID),
  connects to it, and streams the guest framebuffer to the browser over a
  WebSocket, painted on a `<canvas>`.
- VRDE/RDP may require the VirtualBox Extension Pack and a licensing review in
  institutional environments.
- Raw `VBoxManage` stderr is logged server-side and never exposed to the UI.
