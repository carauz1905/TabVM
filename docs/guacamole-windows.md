# Guacamole on Native Windows

TabVM keeps a Windows-native product direction. The local agent, installer, and user-facing setup should be native Windows, without Docker, WSL, or an externally hosted Guacamole server as the recommended MVP path.

The risky part is Apache Guacamole itself: Guacamole is split between a Java web application and `guacd`, a native proxy daemon. The Java web application is portable, but `guacd` and its RDP dependencies are the integration risk on Windows.

## Decision

For MVP 1, TabVM should treat Guacamole as a local managed dependency and validate a native Windows-friendly integration before building deeper product features on top of it.

Recommended MVP direction:

```text
TabVM Local Agent (Go)
  -> manages local VM lifecycle through VBoxManage
  -> configures VirtualBox VRDE/RDP
  -> starts or connects to local Guacamole components
  -> opens browser session for the selected VM
```

Docker, WSL, and external Guacamole instances are useful reference setups, but they are not the target product path.

## What Guacamole requires

According to the Apache Guacamole manual, a native Guacamole installation has two major parts:

| Part | Role | Windows impact |
|------|------|----------------|
| `guacamole-client` | Java web application served by a servlet container, usually Tomcat. | Feasible on Windows because it is Java-based and packaged as a `.war`. |
| `guacamole-server` / `guacd` | Native proxy that connects Guacamole sessions to RDP, VNC, SSH, and other protocols. | High-risk area because it must be built with native dependencies such as Cairo, libjpeg, libpng, libuuid, and FreeRDP for RDP support. |

The official native installation documentation focuses on building `guacamole-server` from source with Unix-style tooling and service integration such as systemd or SysV init. That does not automatically disqualify Windows, but it means Windows-native packaging must be validated explicitly instead of assumed.

## Preflight endpoint

The agent exposes a token-protected preflight endpoint that reports readiness without starting Guacamole:

- `GET http://127.0.0.1:5230/api/guacamole/status` (requires `X-TabVM-Session-Token`)

The response includes:

- `ready`: `true` only when all required components are detected.
- `level`: `missing`, `partial`, `ready`, or `unknown`.
- `components`: presence/configuration booleans and version strings for `java`, `tomcat`, `guacamole.war`, and `guacd`.
- `protocols`: supported protocols (`rdp`, `vnc`, `ssh`).
- `blockers` / `warnings`: sanitized human-readable messages.

Full paths are not exposed in the normal response; only presence, configuration, and version strings are returned.

## Configuration environment variables

For the preflight slice, configure component locations through environment variables:

| Variable | Purpose | Example |
|----------|---------|---------|
| `TABVM_GUACAMOLE_WAR_PATH` | Path to the `guacamole.war` file. | `C:\Program Files\Apache Software Foundation\Tomcat 10.1\webapps\guacamole.war` |
| `TABVM_TOMCAT_HOME` | Root directory of the local Tomcat installation. | `C:\Program Files\Apache Software Foundation\Tomcat 10.1` |
| `TABVM_GUACD_PATH` | Path to the `guacd` executable, or `guacd` to resolve from PATH. | `C:\Tools\guacd\guacd.exe` |
| `TABVM_JAVA_PATH` | Path to the `java` executable, or `java` to resolve from PATH. | `C:\Program Files\Java\jdk-17\bin\java.exe` |
| `TABVM_GUACAMOLE_BASE_URL` | Base URL of the local Guacamole web application. Must be a loopback `http` or `https` URL. | `http://127.0.0.1:8080/guacamole` |
| `TABVM_GUACAMOLE_JSON_SECRET_KEY` | 32-digit hex secret for `guacamole-auth-json`. Replace `<32-hex-secret>` with a generated value. | `<32-hex-secret>` |

The agent reads these values as configuration inputs. They are used only to locate components for presence checks and version checks (`java -version`, `guacd -v`) and to check file or directory existence. No shell command is built from user input, and full resolved paths are not exposed in API responses.

### JSON auth extension requirement

Browser console launch uses the official `guacamole-auth-json` extension. The extension must be installed in Guacamole and configured with the same 32-digit hexadecimal secret key that the agent receives through `TABVM_GUACAMOLE_JSON_SECRET_KEY`.

Generate a new 32-digit hex secret with PowerShell:

```powershell
-join ((1..32) | ForEach-Object { '{0:x}' -f (Get-Random -Maximum 16) })
```

Store the generated value in `TABVM_GUACAMOLE_JSON_SECRET_KEY` and in the `guacamole-auth-json` extension configuration. Do not reuse the placeholder value.

The secret key is treated as a credential:

- It is **never persisted** in the SQLite database.
- It is **never exposed** through API responses, UI, or logs.
- It is used only in memory to sign and encrypt the short-lived JSON auth token.

The token format follows the Apache Guacamole Manual v1.6.0:

1. Build plaintext JSON with `username`, `expires`, and `connections`.
2. Sign the plaintext with HMAC-SHA256 using the 16-byte decoded secret.
3. Prepend the binary signature to the plaintext.
4. Apply PKCS#7 padding and encrypt with AES-128-CBC using a zero IV.
5. Base64-encode the result.

The browser submits the token to Guacamole as a URL-encoded `data` query parameter.

## VM browser console readiness

The agent exposes a combined readiness check for a specific VM:

- `GET http://127.0.0.1:5230/api/vms/{id}/browser-console/readiness` (requires `X-TabVM-Session-Token`)

This endpoint reports whether the VM has a prepared console target and whether Guacamole is ready to consume it. `canOpenBrowserConsole` is `false` until a launch descriptor is generated.

## VM browser console launch

When the preflight components are ready and `TABVM_GUACAMOLE_BASE_URL` and `TABVM_GUACAMOLE_JSON_SECRET_KEY` are configured, the agent can generate a signed Guacamole JSON auth token for a prepared VM:

- `POST http://127.0.0.1:5230/api/vms/{id}/browser-console/launch` (requires `X-TabVM-Session-Token`)

The endpoint returns a launch descriptor:

```json
{
  "canLaunch": true,
  "vmId": "550e8400-e29b-41d4-a716-446655440000",
  "connectionName": "rdp-550e8400-e29b-41d4-a716-446655440000",
  "protocol": "rdp",
  "target": "127.0.0.1:5432",
  "guacamoleBaseUrl": "http://127.0.0.1:8080/guacamole",
  "authData": "base64-encoded-signed-encrypted-json",
  "launchUrl": "http://127.0.0.1:8080/guacamole?data=base64-encoded-signed-encrypted-json",
  "expiresAt": "2026-01-01T00:02:00Z",
  "message": "Browser console launch prepared. Use the link to open Guacamole.",
  "blockers": [],
  "warnings": ["The launch URL and authData are short-lived bearer credentials. Do not log, share, or store them. Browser history and clipboard may retain them."]
}
```

`authData` and `launchUrl` are short-lived bearer credentials. They must not be logged, shared, or stored. Browser history, clipboard history, and browser dev tools can retain them after the session ends, so use them only in the local browser session that opens Guacamole.

If the VM has no prepared console target, Guacamole preflight is not ready, the JSON auth configuration is missing or invalid, or the base URL is not a local `http`/`https` URL, `canLaunch` is `false`, no token is returned, and `blockers` explain why. The secret key is never included in the response.

## MVP validation plan

Before implementing the full console flow, validate this spike:

1. Run Tomcat locally on Windows.
2. Deploy the official `guacamole.war` locally.
3. Determine whether a reliable Windows-native `guacd` path exists for the chosen Guacamole version.
4. Confirm RDP support through FreeRDP-compatible dependencies.
5. Connect Guacamole to a VirtualBox VM exposed through VRDE/RDP on `127.0.0.1`.
6. Document exact install/start/stop steps that TabVM could later automate.

Success means a local Windows machine can open a VirtualBox VM console in the browser through Guacamole without Docker, WSL, or a remote Guacamole server.

## Fallback boundary

If native Windows `guacd` is too fragile for the MVP, TabVM should not silently switch to Docker or WSL. That would violate the product direction.

Instead, the project should pause and choose one of these explicit product decisions:

| Option | Trade-off |
|--------|-----------|
| Keep Guacamole and accept a managed native dependency effort | Preserves browser console goal, but increases packaging complexity. |
| Replace Guacamole for MVP with a Windows-friendly RDP web bridge | Preserves native Windows direction, but changes the console technology. |
| Defer browser console and ship VM control first | Reduces risk, but weakens the core TabVM value proposition. |

No fallback should be treated as automatic. This deserves an explicit architecture decision.

## Acceptance criteria for the spike

- Guacamole web application runs locally on Windows.
- `guacd` or an equivalent local proxy runs locally on Windows.
- RDP support is available.
- A VirtualBox VM with VRDE/RDP enabled can be opened from the browser.
- Startup and shutdown can be scripted reliably.
- The setup can be described well enough to later package with WiX.
- No Docker, WSL, or external Guacamole server is required.

## Sources checked

- Apache Guacamole Manual v1.6.0, native installation: `guacamole-client` is a Java `.war`; `guacamole-server` provides `guacd` and must be built with native dependencies.
- Apache Guacamole Manual v1.6.0, architecture: browser connects to Guacamole web application, which tunnels the Guacamole protocol to `guacd`; `guacd` connects to the remote desktop protocol endpoint.
