# Security Policy

TabVM runs a local agent that performs privileged operations on the host
(controlling VirtualBox, reading/writing VM disks, transferring files into
guests). Security of that boundary is a first-class concern.

## Reporting a vulnerability

Please **do not** open a public issue for security problems.

- Preferred: use GitHub's [private vulnerability reporting](https://docs.github.com/en/code-security/security-advisories/guidance-on-reporting-and-writing-information-about-vulnerabilities/privately-reporting-a-security-vulnerability)
  (the **Report a vulnerability** button under the repository's *Security* tab).
- Alternatively, email **arauz.carlos0587@gmail.com** with details and, if
  possible, a proof of concept.

You can expect an acknowledgement within a few days. Please allow reasonable
time for a fix before any public disclosure.

## Supported versions

TabVM is pre-1.0. Only the latest released version receives security fixes.

| Version | Supported |
| ------- | --------- |
| latest  | ✅        |
| older   | ❌        |

## Security model

The design assumes the local agent is the trusted, privileged boundary:

- The control API binds only to `127.0.0.1` — it is never exposed to the network by default.
- Every `/api/*` request requires a local session token shared between the UI and the agent.
- The UI never receives arbitrary shell execution; only explicit, validated VM operations are allowed.
- VM names, paths, ports, and actions are validated before reaching `VBoxManage`.
- Guest credentials used for `guestcontrol` operations are passed per-operation
  via temporary password files and are never written to argv, logs, or SQLite.
- Sensitive operations (create, import, delete, snapshot restore) are logged.

## Scope

In scope: the desktop agent, the web UI, the build/release scripts, and the
installer. Out of scope: vulnerabilities in VirtualBox itself, the host OS, or
third-party guest operating systems.
