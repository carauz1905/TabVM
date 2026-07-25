# Security Policy

TabVM runs a local agent that performs privileged operations on the host
(controlling VirtualBox, reading/writing VM disks, transferring files into
guests). Security of that boundary is a first-class concern.

## Reporting a vulnerability

Please **do not** open a public issue for security problems.

- Preferred: use GitHub's [private vulnerability reporting](https://docs.github.com/en/code-security/security-advisories/guidance-on-reporting-and-writing-information-about-vulnerabilities/privately-reporting-a-security-vulnerability)
  (the **Report a vulnerability** button under the repository's *Security* tab).
- Alternatively, email **contact@tabvm.com** with details and, if
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

- The control API binds only to `127.0.0.1`, and every request must additionally
  carry a loopback `Host` header. Binding alone is not a boundary: a browser will
  send a request to a loopback port on behalf of any site the user visits, and a
  page whose DNS record is re-pointed at `127.0.0.1` would otherwise be treated
  as same-origin with the agent. An attacker chooses what a name resolves to, not
  the `Host` header the browser sends.
- Every `/api/*` request requires a local session token shared between the UI and
  the agent. WebSocket upgrades additionally require a loopback `Origin`, since
  the same-origin policy does not cover WebSockets.
- The UI document is served with a per-response CSP nonce and `frame-ancestors
  'none'`, so the session token it carries cannot be read by injected script or
  by a framing page.
- The UI never receives arbitrary shell execution; only explicit, validated VM operations are allowed.
- VM names, paths, ports, and actions are validated before reaching `VBoxManage`,
  including a leading-dash guard so a value cannot be parsed as an option.
- Guest credentials used for `guestcontrol` operations are passed per-operation
  via short-lived files and are never written to argv, logs, or SQLite. Those
  files live in the per-user data directory rather than the shared temp
  directory, because Windows ignores the POSIX mode Go requests and applies the
  containing directory's ACL instead.
- Sensitive operations (create, import, delete, snapshot restore) are logged.

## Scope

In scope: the desktop agent, the web UI, the build/release scripts, and the
installer. Out of scope: vulnerabilities in VirtualBox itself, the host OS, or
third-party guest operating systems.
