# TabVM Development Goal

TabVM development should produce a native Windows application that lets students manage and open local VirtualBox virtual machines from a browser with minimal setup friction.

The project must prove the complete local workflow early: detect VirtualBox, list local VMs, start a selected VM headless, expose its console through a browser session, and provide safe basic controls such as stop and reset.

## Primary goal

Build a Windows-native TabVM MVP that validates this end-to-end path:

```text
Browser
  -> React + TypeScript Web UI
  -> Local Go Agent
  -> VBoxManage / VirtualBox
  -> Headless VM with VRDE/RDP
  -> Local Guacamole browser console
```

The first development milestone is successful when a developer can run TabVM locally on Windows and open a registered VirtualBox VM in the browser without Docker, WSL, or an externally hosted Guacamole instance.

## Development principles

- Keep the product Windows-native from the start.
- Build one vertical slice before expanding features.
- Use `VBoxManage` first instead of deeper VirtualBox APIs.
- Treat the local Go agent as the privileged boundary.
- Bind local control APIs to `127.0.0.1` by default.
- Never expose arbitrary shell execution through the UI or API.
- Include Guacamole from the beginning, but validate native Windows operation before depending on it deeply.
- Prefer boring, auditable components over clever abstractions.

## MVP development target

The MVP should include:

- Go local agent.
- React + TypeScript + Vite web UI.
- SQLite local state.
- `VBoxManage` discovery and VM control.
- Apache Guacamole integration for browser console access.
- VirtualBox VRDE/RDP console path.
- Local session token between UI and agent.
- Clear error handling for missing VirtualBox, missing VM, port conflicts, Guacamole startup failure, and VM startup failure.

## Deferred scope

These remain important, but should not block the first vertical slice:

- Final MSI installer.
- Code signing and timestamping.
- Auto-updater.
- Multiple split panels.
- OVA/OVF import.
- Lab templates.
- Advanced snapshot workflows.
- Institution-managed deployment.

## Immediate development sequence

1. Validate native Windows Guacamole feasibility.
2. Scaffold the repository structure for agent, UI, shared contracts, docs, and scripts.
3. Implement the local agent API skeleton with local token validation.
4. Implement `VBoxManage` discovery and VM listing.
5. Build the React UI shell and VM list screen.
6. Add VM start, stop, and reset operations.
7. Integrate the browser console path through Guacamole.
8. Add SQLite persistence for minimal local settings and session metadata.
9. Run a security and reliability pass before expanding scope.

## Success statement

TabVM succeeds when a student can stay in the browser, open a local VM reliably, and control the lab workflow without fighting the VirtualBox desktop window.
