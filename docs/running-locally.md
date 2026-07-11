# Running TabVM Locally on Windows

This document explains how to run the first TabVM scaffold on a Windows development machine.

## Prerequisites

| Tool | Purpose | Download |
|------|---------|----------|
| Go 1.25 or later | Build and run the desktop agent | https://go.dev/dl/ |
| Node.js ^20.19.0 or >=22.12.0 and npm | Build and run the web UI | https://nodejs.org/ |
| VirtualBox (optional) | VM discovery and control | https://www.virtualbox.org/ |

You can verify prerequisites with the included helper:

```powershell
.\scripts\check-prereqs.ps1
```

The script checks for Go 1.25+, Node.js ^20.19.0 or >=22.12.0, npm, the default VirtualBox installation paths, and optional Guacamole preflight reporting. Guacamole preflight is reported as optional and does not block the development launcher; ambient Java alone is not treated as a partial Guacamole setup.

## Repository layout

```text
TabVM/
├─ apps/
│  ├─ desktop-agent/   Go local agent
│  └─ web-ui/          React + TypeScript + Vite UI
├─ packages/
│  └─ shared/          API contract documentation
├─ scripts/            Windows helper scripts
├─ docs/               Project documentation
└─ ...                 No .NET solution or projects
```

## Session token

The agent treats `/api/*` as a privileged boundary. Every request to `/api/*` must include the `X-TabVM-Session-Token` header. `/health` remains unauthenticated and non-sensitive.

`scripts/dev-start.ps1` generates a random per-run development token and exports it to both processes before launching:

- Agent: `TABVM_AGENT_SESSION_TOKEN` and `TabVM__Agent__SessionToken` environment variables.
- Web UI: `VITE_TABVM_SESSION_TOKEN` environment variable.

For a manual agent run or a production build, set the token consistently:

- Agent: `TABVM_AGENT_SESSION_TOKEN` (or the legacy `TabVM__Agent__SessionToken` environment variable).
- Web UI: `VITE_TABVM_SESSION_TOKEN` (must be set at build time for production; use `.env.local` for local overrides).

If `TABVM_AGENT_SESSION_TOKEN` is not configured and the agent runs in Development mode, it generates a one-time random token and logs that a temporary development token is in use. The token value itself is not logged. That generated token is only suitable for local development and must not be used in production; use `scripts/dev-start.ps1` or set `TABVM_AGENT_SESSION_TOKEN` explicitly.

### Local state database configuration

The agent persists minimal local state in a SQLite database. By default the database is created at `%LOCALAPPDATA%\TabVM\tabvm.db`. You can override the location with environment variables:

```powershell
# Use a custom data directory; the database will be <DataDir>\tabvm.db
$env:TABVM_DATA_DIR = "C:\Path\To\TabVM\Data"

# Or point to an explicit database file
$env:TABVM_DB_PATH = "C:\Path\To\tabvm.db"
```

`TABVM_DB_PATH` takes precedence over `TABVM_DATA_DIR`. Paths containing `..` are rejected as a guard against accidental misconfiguration and cause the agent to fail fast at startup with a clear error. The agent creates the parent directory if it does not exist. If the database cannot be opened or initialized, the agent exits during startup; the local-state status endpoint is for runtime availability checks after the agent has started.

What is stored:

- VRDE console port assignments per VM (`vm_console_ports`), so ports remain stable across restarts.
- Non-secret Guacamole component paths/settings (`app_settings`) when persisted by a future management flow. Env vars still take precedence.
- A bounded operation log (`operation_log`) for VM lifecycle and console actions.

What is NOT stored:

- Session tokens, passwords, or other secrets.
- Full VBoxManage paths or host diagnostics.
- Clipboard content or VM display data.

### Guacamole preflight configuration

When running the agent with Guacamole preflight enabled, set the component paths as environment variables:

```powershell
$env:TABVM_GUACAMOLE_WAR_PATH = "C:\Path\To\Tomcat\webapps\guacamole.war"
$env:TABVM_TOMCAT_HOME = "C:\Path\To\Tomcat"
$env:TABVM_GUACD_PATH = "C:\Path\To\guacd.exe"
$env:TABVM_JAVA_PATH = "C:\Path\To\java.exe"
```

### Guacamole JSON auth launch configuration

To generate browser console launch links, also set the Guacamole base URL and the JSON auth secret key:

```powershell
$env:TABVM_GUACAMOLE_BASE_URL = "http://127.0.0.1:8080/guacamole"
$env:TABVM_GUACAMOLE_JSON_SECRET_KEY = "<32-hex-secret>"
```

Generate a new 32-digit hex secret with PowerShell:

```powershell
-join ((1..32) | ForEach-Object { '{0:x}' -f (Get-Random -Maximum 16) })
```

`TABVM_GUACAMOLE_BASE_URL` must be a loopback `http` or `https` URL; non-local hosts and non-http/https schemes are rejected. `TABVM_GUACAMOLE_JSON_SECRET_KEY` must be a 32-digit hexadecimal value and must match the secret configured for the `guacamole-auth-json` extension. The key is treated as a credential: it is not persisted in the SQLite database and is never exposed through API responses or logs.

The `authData` and `launchUrl` values returned by the launch endpoint are short-lived bearer credentials. Do not log, share, or store them. Browser history, clipboard history, and browser dev tools can retain them after the session ends.

Environment variables remain sufficient for local development. If a path is not set via env, the agent may fall back to a previously persisted non-secret setting. Only version checks (`java -version`, `guacd -v`) and file/directory existence checks are performed. No shell command is built from user input.

No real token is committed to source control. `apps/web-ui/.env.development` is intentionally empty. Copy `apps/web-ui/.env.example` to `apps/web-ui/.env.local` and fill in a real token when running the UI manually.

## Start both processes

Run the development launcher from the repository root:

```powershell
.\scripts\dev-start.ps1
```

The launcher performs the following steps in order:

1. Runs `check-prereqs.ps1` and stops if critical tools are missing.
2. Installs frontend dependencies with `npm ci` when `package-lock.json` exists, otherwise `npm install`, and stops if the install fails.
3. Generates a per-run session token and exports it for the agent and the UI.
4. Starts the desktop agent on `http://127.0.0.1:5230` using `go run .`.
5. Verifies readiness through the authenticated discovery endpoint using the per-run token. This avoids treating an old process on port `5230` as the newly launched agent.
6. Starts the Vite dev server on `http://localhost:5173` (proxies `/api` and `/health` to the agent).

Open `http://localhost:5173` in your browser.

## Start the agent manually

```powershell
cd apps\desktop-agent
$env:TABVM_AGENT_SESSION_TOKEN = "your-dev-token"
go run .
```

The agent binds to localhost only. The default address is `127.0.0.1` and is overridden with `TABVM_AGENT_BIND_ADDRESS` and `TABVM_AGENT_BIND_PORT`. For security, `TABVM_AGENT_BIND_ADDRESS` must be a loopback address such as `127.0.0.1`, `::1`, or `localhost`. Values like `0.0.0.0` or any LAN IP are rejected at startup unless an explicit unsafe-binding flag is added in the future.

Available endpoints:

- `GET http://127.0.0.1:5230/health` (no token required)
- `GET http://127.0.0.1:5230/api/vbox/discovery` (requires `X-TabVM-Session-Token`)
- `GET http://127.0.0.1:5230/api/console/protocols` (requires `X-TabVM-Session-Token`; returns supported console protocols and auto-configure capability metadata)
- `GET http://127.0.0.1:5230/api/guacamole/status` (requires `X-TabVM-Session-Token`; returns native Windows Guacamole component preflight status)
- `GET http://127.0.0.1:5230/api/local-state/status` (requires `X-TabVM-Session-Token`; returns whether the SQLite local state database is configured/available and its schema version, without exposing the filesystem path)
- `GET http://127.0.0.1:5230/api/vms` (requires `X-TabVM-Session-Token`; returns best-effort real status when `VBoxManage list runningvms` succeeds; VMs that are not running are reported as `not running` rather than `powered off`)
- `GET http://127.0.0.1:5230/api/vms/{id}/status` (requires `X-TabVM-Session-Token`)
- `GET http://127.0.0.1:5230/api/vms/{id}/console` (requires `X-TabVM-Session-Token`; returns console status including `protocol`, `source`, `targets`, and readiness)
- `GET http://127.0.0.1:5230/api/vms/{id}/browser-console/readiness` (requires `X-TabVM-Session-Token`; combines VM console target readiness with Guacamole preflight status; `canOpenBrowserConsole` is `false` until a launch descriptor is generated)
- `POST http://127.0.0.1:5230/api/vms/{id}/browser-console/launch` (requires `X-TabVM-Session-Token`; generates a short-lived signed Guacamole JSON auth token and returns a launch descriptor; requires a loopback `TABVM_GUACAMOLE_BASE_URL` and `TABVM_GUACAMOLE_JSON_SECRET_KEY`; the returned `authData`/`launchUrl` are bearer credentials and must not be logged, shared, or stored)
- `POST http://127.0.0.1:5230/api/vms/{id}/start` (requires `X-TabVM-Session-Token`; runs `VBoxManage startvm {id} --type headless`)
- `POST http://127.0.0.1:5230/api/vms/{id}/stop` (requires `X-TabVM-Session-Token`; runs `VBoxManage controlvm {id} acpipowerbutton`)
- `POST http://127.0.0.1:5230/api/vms/{id}/reset` (requires `X-TabVM-Session-Token`; runs `VBoxManage controlvm {id} reset`)
- `POST http://127.0.0.1:5230/api/vms/{id}/console/prepare` (requires `X-TabVM-Session-Token`; runs `VBoxManage modifyvm {id} --vrde on --vrdeaddress 127.0.0.1 --vrdeport <port>` and produces an `rdp` target with `source=virtualbox-vrde`)
- `POST http://127.0.0.1:5230/api/vms/{id}/console/disable` (requires `X-TabVM-Session-Token`; runs `VBoxManage modifyvm {id} --vrde off`)

Only UUID-shaped VM identifiers are accepted; arbitrary strings are rejected before they reach `VBoxManage`. Concurrent lifecycle operations on the same VM are rejected with `409 Conflict`. Operation failures are logged server-side with full `VBoxManage` stderr but returned to the UI as sanitized messages.

## Start the web UI manually

```powershell
cd apps\web-ui
copy .env.example .env.local
# Edit .env.local and set VITE_TABVM_SESSION_TOKEN to match the agent token
npm install
npm run dev
```

The dev server proxies `/api` and `/health` to the agent so the browser can call the local API without CORS issues.

## Build for production

### Agent

```powershell
cd apps\desktop-agent
go build -o tabvm-agent.exe .
```

### Web UI

```powershell
cd apps\web-ui
npm install
npm run build
```

## Testing

### Web UI

```powershell
cd apps\web-ui
npm install
npm run test
```

Focused tests (`it.only` / `describe.only`) are rejected by the Vitest configuration and will cause the test run to fail.

### Desktop agent

```powershell
cd apps\desktop-agent
go test ./...
```

If Go 1.25+ is not installed, the agent tests cannot run. Install Go from https://go.dev/dl/ to execute them.

## Notes

- The agent does not expose arbitrary shell execution.
- VM lifecycle endpoints are write operations protected by the same session token as read endpoints.
- Stop uses `acpipowerbutton` for a graceful ACPI shutdown request. If a VM does not respond to ACPI, use reset or shut the VM down inside the guest.
- Operational failures from `VBoxManage` are no longer silently collapsed into an empty VM list; the API returns an explicit error response when VirtualBox is missing or when `VBoxManage` fails.
- Raw `VBoxManage` stderr is logged server-side and never exposed to the UI, so host-sensitive paths or details do not leak through the API.
- Console preparation configures VirtualBox VRDE/RDP on `127.0.0.1`. Port selection starts from a deterministic candidate in the range `5000-5999` derived from the VM ID. It skips ports already used by another VM's VRDE server and, where practical, ports that are already bound locally, probing forward to the next available port. This produces the first auto-configured console target (`protocol=rdp`, `source=virtualbox-vrde`) for future Guacamole integration; it is not the final embedded browser console.
- The agent models console targets as protocol-capable endpoints. Supported Guacamole protocols include `rdp`, `vnc`, and `ssh`. Only RDP/VRDE is auto-configured in this slice; VNC and SSH require guest or host service configuration in a later slice and are exposed only as capability metadata.
- VRDE/RDP support may require the VirtualBox Extension Pack and a licensing review in institutional environments. If `VBoxManage` rejects VRDE commands, the API returns a sanitized error message.
- Guacamole browser console launch uses signed JSON auth tokens. The `guacamole-auth-json` extension must be installed and configured with the same 32-digit hex secret as `TABVM_GUACAMOLE_JSON_SECRET_KEY`. The secret key is never persisted in the database or exposed through the API.
