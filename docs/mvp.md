# TabVM MVP 1: Vertical Slice

MVP 1 proves the core TabVM workflow: a student can open a local VirtualBox VM from a browser through a local Windows agent and Guacamole.

This is not the final product. It is the smallest useful version that validates the architecture end to end.

## Goal

Open a local VirtualBox VM in the browser with a repeatable local setup.

The first release should prove this flow:

```text
List local VMs -> Start selected VM headless -> Enable/use VRDE/RDP -> Open console in browser -> Stop/reset VM
```

## Included

| Area | MVP scope |
|------|-----------|
| Local agent | Go service or local process bound to `127.0.0.1`. |
| Web UI | React + TypeScript + Vite app for listing and opening VMs. |
| Local state | SQLite for minimal settings, VM metadata cache, and session records. |
| VirtualBox control | `VBoxManage` discovery, VM listing, VM status, start, stop, reset, and VRDE/RDP configuration. |
| Console access | Apache Guacamole from the start, connected to the VM through RDP/VRDE. |
| Security | Local-only API, local session token, explicit allowlist of VM operations, no arbitrary shell execution. |
| Developer setup | Documented local dev flow for agent, UI, Guacamole, and VirtualBox. |

## Not included yet

- Final MSI installer.
- Code signing and timestamping.
- Auto-updater.
- Multiple simultaneous split panels.
- OVA/OVF import.
- Lab templates.
- Advanced snapshot workflows.
- Institution-managed deployment through Intune, GPO, SCCM, or MSI policy.

## Core user flow

1. User opens TabVM in the browser.
2. UI connects to the local TabVM agent.
3. Agent validates the local session token.
4. Agent discovers VirtualBox and `VBoxManage`.
5. UI shows registered local VMs and status.
6. User starts one VM.
7. Agent starts the VM headless and prepares VRDE/RDP access.
8. UI opens a Guacamole session in the browser.
9. User can stop or reset the VM from the UI.

## Component responsibilities

### TabVM Local Agent

- Own privileged host operations.
- Bind the API to `127.0.0.1` by default.
- Locate VirtualBox and `VBoxManage`.
- Execute only explicit VM operations.
- Validate VM identifiers, ports, and actions.
- Store minimal local state in SQLite.
- Log sensitive operations.

### TabVM Web UI

- Present VM list and VM status.
- Start, open, stop, and reset a selected VM.
- Show clear setup and runtime errors.
- Embed or launch the Guacamole browser session.
- Avoid exposing privileged implementation details.

### Guacamole bridge

- Provide browser access to the VM console.
- Connect to VirtualBox VRDE/RDP for the first MVP.
- Keep protocol complexity outside the React UI.

## Acceptance criteria

- A developer can run the local agent and web UI from the repository.
- The agent can detect whether VirtualBox and `VBoxManage` are available.
- The UI can list registered VirtualBox VMs.
- A selected VM can be started headless.
- The VM console can be opened in the browser through Guacamole.
- The VM can be stopped or reset from the UI.
- The local API is not reachable from the network by default.
- No endpoint allows arbitrary shell command execution.

## First implementation order

1. Repository scaffold for agent, UI, shared contracts, docs, and scripts.
2. Agent API skeleton with local token validation.
3. `VBoxManage` discovery and VM listing.
4. React UI shell and VM list screen.
5. Start/stop/reset VM operations.
6. Guacamole development integration.
7. Browser console launch flow.
8. Minimal SQLite persistence.
9. Security and error handling pass.

## Next decision

Define and validate the native Windows development topology for Guacamole during MVP work. Docker, WSL, and external Guacamole instances are not part of the recommended MVP path. See [`guacamole-windows.md`](guacamole-windows.md).
