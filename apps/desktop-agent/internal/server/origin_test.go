package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tabvm/desktop-agent/internal/config"
)

// foreignRequest builds a request the way a DNS-rebinding attacker's page would:
// the browser resolves the attacker's hostname to 127.0.0.1, so the request
// arrives on loopback but still carries the attacker's authority in Host.
func foreignRequest(method, target string) *http.Request {
	req := httptest.NewRequest(method, target, nil)
	req.Host = "evil.example.com:5230"
	req.Header.Set("Origin", "http://evil.example.com")
	return req
}

// The index page carries the session token, so a foreign Host must never get a
// 200 out of it. This is the exact request a rebinding attacker issues first.
func TestForeignHostNeverReceivesTheSessionToken(t *testing.T) {
	const token = "SUPER-SECRET-TOKEN"
	srv, _ := newTestServer(t, token)
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, foreignRequest(http.MethodGet, "/"))

	if rr.Code != http.StatusMisdirectedRequest {
		t.Fatalf("expected status %d for a foreign Host, got %d", http.StatusMisdirectedRequest, rr.Code)
	}
	if strings.Contains(rr.Body.String(), token) {
		t.Fatal("session token was served to a foreign Host")
	}
}

// The Host check must wrap the whole mux, not just /api. /health and the static
// UI are both reachable without a token, so leaving either outside the check
// would hand a rebinding attacker a foothold.
func TestForeignHostIsRejectedOnEveryRoute(t *testing.T) {
	srv, _ := newTestServer(t, "secret")

	for _, target := range []string{"/", "/health", "/api/vms", "/assets/index.js"} {
		t.Run(target, func(t *testing.T) {
			rr := httptest.NewRecorder()
			srv.Handler().ServeHTTP(rr, foreignRequest(http.MethodGet, target))

			if rr.Code != http.StatusMisdirectedRequest {
				t.Fatalf("expected status %d, got %d", http.StatusMisdirectedRequest, rr.Code)
			}
		})
	}
}

// A valid token must not rescue a foreign Host: the check runs before auth, so
// even a leaked token cannot be replayed from a rebound origin.
func TestForeignHostIsRejectedEvenWithAValidToken(t *testing.T) {
	srv, _ := newTestServer(t, "secret")
	req := foreignRequest(http.MethodGet, "/api/vms")
	req.Header.Set(sessionTokenHeader, "secret")
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusMisdirectedRequest {
		t.Fatalf("expected status %d, got %d", http.StatusMisdirectedRequest, rr.Code)
	}
}

// Every authority the local UI and the launcher actually use must pass.
func TestLoopbackHostsAreAccepted(t *testing.T) {
	srv, _ := newTestServer(t, "secret")

	hosts := []string{
		"127.0.0.1:5230",
		"localhost:5230",
		"[::1]:5230",
		"127.0.0.1",
		"LOCALHOST:5230",
	}
	for _, host := range hosts {
		t.Run(host, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			req.Host = host
			rr := httptest.NewRecorder()

			srv.Handler().ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("expected loopback Host %q to be served, got status %d", host, rr.Code)
			}
		})
	}
}

// checkStreamOrigin gates the screen and serial WebSocket upgrades, which the
// same-origin policy does not cover. An empty Origin is a non-browser client and
// stays gated by the session token alone.
func TestCheckStreamOrigin(t *testing.T) {
	cases := []struct {
		name    string
		origin  string
		devMode bool
		want    bool
	}{
		{name: "no origin header (non-browser client)", origin: "", want: true},
		{name: "agent's own origin", origin: "http://127.0.0.1:5230", want: true},
		{name: "localhost on the bind port", origin: "http://localhost:5230", want: true},
		{name: "ipv6 loopback", origin: "http://[::1]:5230", want: true},
		{name: "attacker origin", origin: "http://evil.example.com", want: false},
		{name: "attacker origin on the bind port", origin: "http://evil.example.com:5230", want: false},
		{name: "loopback on another port in production", origin: "http://127.0.0.1:9999", want: false},
		{name: "vite dev server in production", origin: "http://localhost:5173", want: false},
		{name: "vite dev server in development", origin: "http://localhost:5173", devMode: true, want: true},
		{name: "malformed origin", origin: "://nonsense", want: false},
		{name: "null origin from a sandboxed frame", origin: "null", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := "Production"
			if tc.devMode {
				env = "Development"
			}
			srv := New(&config.Agent{BindAddress: "127.0.0.1", BindPort: 5230, SessionToken: "secret", Environment: env}, nil, nil, nil)

			req := httptest.NewRequest(http.MethodGet, "/api/vms/x/screen-stream", nil)
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}

			if got := srv.checkStreamOrigin(req); got != tc.want {
				t.Fatalf("checkStreamOrigin(%q) = %v, want %v", tc.origin, got, tc.want)
			}
		})
	}
}
