package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

// serveUIDocument fetches the UI document the way the local browser does.
func serveUIDocument(t *testing.T, srv *Server) *httptest.ResponseRecorder {
	t.Helper()
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, newTestRequest(http.MethodGet, "/", nil))
	return rr
}

func TestUIDocumentCarriesASecurityPolicy(t *testing.T) {
	srv, _ := newTestServer(t, "secret")
	csp := serveUIDocument(t, srv).Header().Get("Content-Security-Policy")

	if csp == "" {
		t.Fatal("expected a Content-Security-Policy header on the UI document")
	}

	// The directives that actually contain the damage if the page is ever
	// injected into, or framed by, something hostile.
	for _, directive := range []string{
		"frame-ancestors 'none'",
		"base-uri 'none'",
		"form-action 'none'",
		"object-src 'none'",
		"default-src 'self'",
	} {
		if !strings.Contains(csp, directive) {
			t.Errorf("expected CSP to contain %q, got %q", directive, csp)
		}
	}
}

// A nonce-based script-src is the whole point: the session token lives in the
// DOM, so 'unsafe-inline' would leave it readable by anything injected.
func TestScriptPolicyUsesANonceNotUnsafeInline(t *testing.T) {
	srv, _ := newTestServer(t, "secret")
	csp := serveUIDocument(t, srv).Header().Get("Content-Security-Policy")

	scriptSrc := ""
	for _, d := range strings.Split(csp, ";") {
		if strings.HasPrefix(strings.TrimSpace(d), "script-src") {
			scriptSrc = strings.TrimSpace(d)
		}
	}
	if scriptSrc == "" {
		t.Fatalf("no script-src directive in %q", csp)
	}
	if strings.Contains(scriptSrc, "'unsafe-inline'") {
		t.Errorf("script-src must not allow 'unsafe-inline', got %q", scriptSrc)
	}
	if !strings.Contains(scriptSrc, "'nonce-") {
		t.Errorf("script-src must carry a nonce, got %q", scriptSrc)
	}
}

// A nonce reused across responses is no better than 'unsafe-inline'.
func TestNonceIsFreshPerResponse(t *testing.T) {
	srv, _ := newTestServer(t, "secret")
	nonceOf := regexp.MustCompile(`'nonce-([^']+)'`)

	first := nonceOf.FindStringSubmatch(serveUIDocument(t, srv).Header().Get("Content-Security-Policy"))
	second := nonceOf.FindStringSubmatch(serveUIDocument(t, srv).Header().Get("Content-Security-Policy"))

	if first == nil || second == nil {
		t.Fatal("expected a nonce in both responses")
	}
	if first[1] == second[1] {
		t.Fatalf("nonce was reused across responses: %q", first[1])
	}
}

// Every inline script in the served document must carry the response's nonce,
// or the CSP silently breaks the app instead of protecting it. The placeholder
// the template ships with must be fully consumed.
func TestEveryInlineScriptIsNonced(t *testing.T) {
	srv, _ := newTestServer(t, "secret")
	rr := serveUIDocument(t, srv)
	body := rr.Body.String()

	if strings.Contains(body, inlineScriptNonceMarker) {
		t.Errorf("template placeholder %q survived into the response", inlineScriptNonceMarker)
	}

	nonce := regexp.MustCompile(`'nonce-([^']+)'`).
		FindStringSubmatch(rr.Header().Get("Content-Security-Policy"))
	if nonce == nil {
		t.Fatal("expected a nonce in the policy")
	}

	// Any opening <script> tag without a src is inline and needs the nonce.
	for _, tag := range regexp.MustCompile(`<script[^>]*>`).FindAllString(body, -1) {
		if strings.Contains(tag, "src=") {
			continue
		}
		if !strings.Contains(tag, `nonce="`+nonce[1]+`"`) {
			t.Errorf("inline script tag is missing the response nonce: %s", tag)
		}
	}
}

func TestStaticResponsesRefuseMimeSniffing(t *testing.T) {
	srv, _ := newTestServer(t, "secret")
	rr := serveUIDocument(t, srv)

	if got := rr.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("expected nosniff, got %q", got)
	}
	if got := rr.Header().Get("Referrer-Policy"); got != "no-referrer" {
		t.Errorf("expected no-referrer, got %q", got)
	}
}

// The console dials ws:// on the agent's own port. If connect-src does not
// permit it the stream dies silently, so pin it.
func TestPolicyPermitsTheConsoleWebSocket(t *testing.T) {
	srv, _ := newTestServer(t, "secret")
	csp := serveUIDocument(t, srv).Header().Get("Content-Security-Policy")

	if !strings.Contains(csp, "ws://127.0.0.1:5230") {
		t.Errorf("expected connect-src to permit the agent's WebSocket origin, got %q", csp)
	}
}

// Shutdown must be safe on a server that never started listening; the tray can
// quit before ListenAndServe has assigned the server.
func TestShutdownBeforeListenIsANoop(t *testing.T) {
	srv, _ := newTestServer(t, "secret")

	if err := srv.Shutdown(context.Background()); err != nil {
		t.Fatalf("expected Shutdown to be a no-op before listening, got %v", err)
	}
}
