# TabVM Shared Contracts

Cross-cutting notes about the API surface shared between the desktop agent and
the web UI.

> [!NOTE]
> The agent's route set is the source of truth. This document describes the
> stable core and the endpoint groups at a high level; it is not an exhaustive,
> versioned contract. See `apps/desktop-agent/internal/server` and the typed
> client in `apps/web-ui/src/api/client.ts`.

## Boundary

The local agent exposes a narrow HTTP API bound to `127.0.0.1` by default. Every
`/api/*` request requires the `X-TabVM-Session-Token` header. `/health` is
unauthenticated and non-sensitive.

## Stable core endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Agent liveness and software `version`. |
| GET | `/api/vbox/discovery` | VirtualBox / VBoxManage discovery status. |
| GET | `/api/local-state/status` | SQLite local state configured/available status and schema version. |
| GET | `/api/vms` | List registered VirtualBox VMs with best-effort real state. |
| GET | `/api/vms/{id}/status` | Detailed state for a single VM. |
| POST | `/api/vms/{id}/start` | Start the VM headlessly. |
| POST | `/api/vms/{id}/stop` | Request an ACPI shutdown. |
| POST | `/api/vms/{id}/reset` | Forcibly reset the VM. |
| GET | `/api/vms/{id}/console` | Console status: `protocol`, `source`, `targets`, readiness. |
| POST | `/api/vms/{id}/console/prepare` | Enable local-only VRDE/RDP on a deterministic loopback port. |
| POST | `/api/vms/{id}/console/disable` | Disable VRDE/RDP. |

Additional VM operations are dispatched under `/api/vms/{id}/…` — telemetry,
snapshots, networking, hardware (vCPU / memory), storage (resize / add / detach),
shared folders, clipboard, Guest Additions, and file transfer — plus VM creation
(`/api/vms/create`, `/api/vms/import`) and host pickers (`/api/host/pick-file`,
`/api/host/pick-folder`).

## Console model

The console is a **direct** stream: the agent enables VirtualBox VRDE/RDP on
`127.0.0.1`, connects to it, and streams the guest framebuffer to the browser
over a WebSocket, painted on a `<canvas>`. Only RDP through VirtualBox VRDE is
used; console targets are modeled as protocol-capable endpoints, but the browser
console does not depend on any external gateway.

### `VmConsoleStatusResponse`

```json
{
  "id": "uuid",
  "enabled": true,
  "protocol": "rdp",
  "source": "virtualbox-vrde",
  "address": "127.0.0.1",
  "port": 5432,
  "ready": true,
  "target": "127.0.0.1:5432",
  "targets": [
    {
      "protocol": "rdp",
      "host": "127.0.0.1",
      "port": 5432,
      "source": "virtualbox-vrde",
      "displayName": "VirtualBox VRDE/RDP",
      "ready": true
    }
  ],
  "message": ""
}
```

## Notes

- All API routes are local-only by default and require the session token
  (except `/health`).
- The local agent is implemented in Go. No endpoint accepts arbitrary shell
  commands; VM identifiers, ports, and actions are validated before reaching
  `VBoxManage`.
- Discovery and status responses intentionally omit host-side path details.
- Future versions may add OpenAPI generation and a typed client package.
