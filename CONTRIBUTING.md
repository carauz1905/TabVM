# Contributing to TabVM

Thanks for your interest in improving TabVM. This guide covers the local setup,
the build and test workflow, and the conventions we follow.

## Contributor License Agreement

Before your first contribution can be merged, you must accept the
[Contributor License Agreement (CLA)](CLA.md). TabVM is © Beacon Solutions and is
[dual-licensed](README.md#license) under the AGPL-3.0 and separate commercial
licenses; the CLA grants Beacon Solutions the rights needed to offer TabVM under
both.

You do not need to sign anything by hand: when you open a pull request, the
automated CLA check asks you to confirm acceptance, and your GitHub identity is
recorded as your signature. Corporate contributors should email
`legal@tabvm.com` to arrange a corporate CLA first.

## Prerequisites

- **Oracle VirtualBox** installed (with `VBoxManage` on the host).
- **Node.js** `^20.19 || >=22.12` and npm.
- **Go** `>=1.26`.
- **Windows** is the primary target host; the agent and UI build on other
  platforms, but VM operations require VirtualBox.

## Getting started

```bash
# 1. Install workspace dependencies
npm ci

# 2. Build everything (web UI -> embedded into the agent -> agent binary)
npm run build
```

> [!IMPORTANT]
> The agent's embedded web UI (`apps/desktop-agent/internal/webui/dist`) is a
> **generated** artifact. It is git-ignored and required at compile time by
> `//go:embed`, so you must run `npm run build` (or at least
> `npm run webui:build && node scripts/embed-ui.mjs`) before `go build` or
> `go test` on a fresh clone.

## Running the tests

```bash
npm test                 # web UI (vitest) + agent (go test)
npm run webui:test       # web UI only
npm run agent:test       # agent only
npm run webui:typecheck  # TypeScript type check
```

## Project layout

```text
apps/
  desktop-agent/   Go local agent (VBoxManage wrapper, HTTP API, embedded UI)
  web-ui/          React + TypeScript + Vite front-end
packages/
  shared/          Shared code
scripts/           Build, release, and dev helpers
docs/              Architecture, security, and workflow docs
installer/         Inno Setup script
```

## Branching workflow

We favor branching over forking for regular contributors. Features are developed
on their own branch — worktrees keep `main` clean and buildable:

```bash
git worktree add ../TabVM-<feature> feat/<feature>
```

Keep commits small and focused. Rebase on `main` before opening a pull request.

## Commit messages

We use [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<optional scope>): <description>
```

Common types: `feat`, `fix`, `docs`, `refactor`, `perf`, `test`, `build`, `ci`,
`chore`. Do not add AI attribution or `Co-Authored-By` trailers.

## Tests and TDD

New behavior ships with tests. The agent's `VBoxManage` calls are covered with
fake runners (no real VirtualBox needed); the UI is covered with vitest.

## Language and style

- Code, comments, identifiers, UI copy, and docs default to **English**.
- Spanish UI strings live in `apps/web-ui/src/i18n/es.ts` and use neutral,
  non-regional Spanish.
