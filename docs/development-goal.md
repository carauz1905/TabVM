# Project Goals & Principles

TabVM is a native Windows-first application that lets students manage and open
local VirtualBox virtual machines from a browser with minimal setup friction.

## Goal

Let a student stay in the browser — open a local VM reliably and control the lab
workflow — without fighting the VirtualBox desktop window.

```text
Browser
  -> React + TypeScript Web UI
  -> Local Go Agent
  -> VBoxManage / VirtualBox
  -> Headless VM with VRDE/RDP
  -> Guest framebuffer streamed to a browser <canvas>
```

## Principles

- Keep the product Windows-native.
- Use `VBoxManage` first instead of deeper VirtualBox APIs.
- Treat the local Go agent as the privileged boundary.
- Bind local control APIs to `127.0.0.1` by default.
- Never expose arbitrary shell execution through the UI or API.
- Prefer boring, auditable components over clever abstractions.
- Ship features as focused vertical slices with tests.

## Success statement

TabVM succeeds when a student can stay in the browser, open a local VM reliably,
and control the lab workflow without fighting the VirtualBox desktop window.

See [`architecture.md`](architecture.md) for the technical design and the
project [`README`](../README.md) for the current feature set and roadmap.
