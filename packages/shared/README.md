# TabVM Shared Contracts

This package holds cross-cutting API contracts, schemas, and documentation used by both the desktop agent and the web UI.

## API Contract

The local agent exposes a narrow HTTP API bound to `127.0.0.1` by default.

### Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Agent liveness. |
| GET | `/api/vbox/discovery` | VirtualBox / VBoxManage discovery status. |
| GET | `/api/console/protocols` | Supported console protocols and auto-configure capability metadata. |
| GET | `/api/guacamole/status` | Native Windows Guacamole component preflight status. |
| GET | `/api/local-state/status` | SQLite local state configured/available status and schema version. |
| GET | `/api/vms` | List registered VirtualBox VMs. |
| GET | `/api/vms/{id}/status` | Detailed state for a single VM. |
| GET | `/api/vms/{id}/console` | Console status including protocol, source, targets, and readiness. |
| POST | `/api/vms/{id}/start` | Start the VM headlessly. |
| POST | `/api/vms/{id}/stop` | Request an ACPI shutdown. |
| POST | `/api/vms/{id}/reset` | Forcibly reset the VM. |
| POST | `/api/vms/{id}/console/prepare` | Enable local-only VRDE/RDP for Guacamole. |
| POST | `/api/vms/{id}/console/disable` | Disable VRDE/RDP. |
| GET | `/api/vms/{id}/browser-console/readiness` | Combine VM console target readiness with Guacamole preflight status. |
| POST | `/api/vms/{id}/browser-console/launch` | Generate a short-lived signed Guacamole JSON auth token and launch descriptor. |

### Models

#### `VirtualBoxDiscovery`

```json
{
  "found": true,
  "version": "7.0.14r161095",
  "error": null
}
```

> The discovery response intentionally omits the resolved `VBoxManage` path.
> Host-side path details should only be exposed through a future authenticated
> `/api/vbox/diagnostics` endpoint, not the normal discovery response.

#### `VmListResponse`

```json
{
  "vms": [
    { "id": "uuid", "name": "VM Name", "state": "listed" }
  ]
}
```

#### `VmStatusResponse`

```json
{
  "id": "uuid",
  "state": "running"
}
```

#### `VmOperationResponse`

```json
{
  "success": true,
  "vmId": "uuid",
  "message": "VM start requested."
}
```

#### `ConsoleProtocolsResponse`

```json
{
  "protocols": [
    {
      "id": "rdp",
      "displayName": "RDP",
      "canAutoConfigure": true,
      "description": "E.g., auto-configured through VirtualBox VRDE on the loopback interface."
    },
    {
      "id": "vnc",
      "displayName": "VNC",
      "canAutoConfigure": false,
      "description": "E.g., supported by Guacamole; requires a guest or host VNC service configured in a future slice."
    },
    {
      "id": "ssh",
      "displayName": "SSH",
      "canAutoConfigure": false,
      "description": "E.g., supported by Guacamole; requires a guest SSH service configured in a future slice."
    }
  ]
}
```

> Protocol `description` values above are examples for documentation purposes.
> They are not a stable contract and may differ from the running agent's
> `/api/console/protocols` response. Use that endpoint as the source of truth.

#### `GuacamoleStatusResponse`

```json
{
  "ready": false,
  "level": "missing",
  "components": [
    { "name": "java", "present": false, "configured": false },
    { "name": "tomcat", "present": false, "configured": false },
    { "name": "guacamole.war", "present": false, "configured": false },
    { "name": "guacd", "present": false, "configured": false }
  ],
  "protocols": ["rdp", "vnc", "ssh"],
  "blockers": ["java is not configured."],
  "warnings": ["Native Windows guacd with RDP support must be validated separately..."]
}
```

> The response intentionally omits full file paths. Host-side path details
> should only be exposed through a future authenticated diagnostics endpoint.

#### `LocalStateStatusResponse`

```json
{
  "configured": true,
  "available": true,
  "schema": 1
}
```

> The response reports whether the SQLite local state database is configured
> and reachable, plus the applied schema version. It intentionally omits the
> resolved filesystem path.

#### `VmBrowserConsoleReadinessResponse`

```json
{
  "vmId": "uuid",
  "consoleReady": true,
  "guacamoleReady": false,
  "guacamoleLevel": "missing",
  "targetAvailable": true,
  "protocol": "rdp",
  "target": "127.0.0.1:5432",
  "canOpenBrowserConsole": false,
  "message": "Browser console pending Guacamole setup.",
  "blockers": ["java is not configured."],
  "warnings": ["Native Windows guacd with RDP support must be validated separately."]
}
```

#### `VmBrowserConsoleLaunchResponse`

```json
{
  "canLaunch": true,
  "vmId": "uuid",
  "connectionName": "rdp-uuid",
  "protocol": "rdp",
  "target": "127.0.0.1:5432",
  "guacamoleBaseUrl": "http://127.0.0.1:8080/guacamole",
  "authData": "base64-encoded-signed-encrypted-json",
  "launchUrl": "http://127.0.0.1:8080/guacamole?data=base64-encoded-signed-encrypted-json",
  "expiresAt": "2026-01-01T00:02:00Z",
  "message": "Browser console launch prepared. Use the link to open Guacamole.",
  "blockers": [],
  "warnings": ["The JSON auth token is short-lived and intended for the browser to send to Guacamole."]
}
```

> The secret key is never included in this response. `authData` and `launchUrl` are short-lived bearer credentials intended for the local browser session to submit to Guacamole. They must not be logged, shared, or stored. Browser history, clipboard history, and browser dev tools can retain them after the session ends.

#### `VmConsoleStatusResponse`

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

#### `VmConsoleOperationResponse`

```json
{
  "success": true,
  "vmId": "uuid",
  "message": "VRDE console disabled."
}
```

## Notes

- All API routes are local-only by default.
- The local agent is implemented in Go.
- No endpoint accepts arbitrary shell commands.
- Console targets are protocol-aware. Supported Guacamole protocols include `rdp`, `vnc`, and `ssh`.
- Only RDP through VirtualBox VRDE is auto-configured in this slice. VNC and SSH are exposed as capability metadata and require guest or host service configuration in a later slice.
- Protocol `description` strings are illustrative and may change as the agent evolves; consume the `/api/console/protocols` endpoint for current values.
- Future versions may add OpenAPI generation and typed client sharing.
