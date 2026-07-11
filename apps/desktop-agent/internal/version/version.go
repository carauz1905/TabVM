// Package version holds the single source of truth for the TabVM agent's
// release version. Bump this string on each release; the value is surfaced to
// the web UI through GET /health so the sidebar can display it.
package version

// Version is the current TabVM release. Keep it in sync with
// apps/web-ui/package.json.
const Version = "0.1.0"
