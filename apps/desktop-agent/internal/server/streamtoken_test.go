package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

func TestStreamTokenFromRequest(t *testing.T) {
	cases := []struct {
		name     string
		protocol string
		query    string
		want     string
	}{
		{name: "subprotocol", protocol: "tabvm.token.abc123", want: "abc123"},
		{name: "subprotocol among others", protocol: "chat, tabvm.token.abc123", want: "abc123"},
		{name: "subprotocol with padding", protocol: "  tabvm.token.abc123  ", want: "abc123"},
		{name: "legacy query parameter", query: "?token=abc123", want: "abc123"},
		{name: "subprotocol wins over query", protocol: "tabvm.token.fromproto", query: "?token=fromquery", want: "fromproto"},
		{name: "unrelated subprotocol only", protocol: "chat", want: ""},
		{name: "nothing offered", want: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/vms/x/screen-stream"+tc.query, nil)
			if tc.protocol != "" {
				req.Header.Set("Sec-WebSocket-Protocol", tc.protocol)
			}

			if got := streamTokenFromRequest(req); got != tc.want {
				t.Fatalf("streamTokenFromRequest() = %q, want %q", got, tc.want)
			}
		})
	}
}

// A browser aborts the connection if the server does not select one of the
// subprotocols it offered, so the echo is load-bearing rather than cosmetic.
// This drives a real gorilla handshake over a real socket, which is the only
// way to know the negotiation actually works without a running VM.
func TestWebSocketHandshakeNegotiatesTheTokenSubprotocol(t *testing.T) {
	const token = "0123456789abcdef"
	srv, _ := newTestServer(t, token)

	// Mirror what the stream handlers do after their VM-specific setup.
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if streamTokenFromRequest(r) != token {
			http.Error(w, "Invalid or missing session token.", http.StatusUnauthorized)
			return
		}
		upgrader := srv.streamUpgrader()
		conn, err := upgrader.Upgrade(w, r, streamResponseHeader(r))
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.WriteMessage(websocket.TextMessage, []byte("ready"))
	}))
	defer backend.Close()

	wsURL := "ws" + strings.TrimPrefix(backend.URL, "http")
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, http.Header{
		"Sec-WebSocket-Protocol": []string{streamTokenSubprotocolPrefix + token},
	})
	if err != nil {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		t.Fatalf("handshake failed (status %d): %v", status, err)
	}
	defer conn.Close()

	if got := conn.Subprotocol(); got != streamTokenSubprotocolPrefix+token {
		t.Fatalf("server selected subprotocol %q, want the token-bearing one", got)
	}

	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("reading the first frame failed: %v", err)
	}
	if string(msg) != "ready" {
		t.Fatalf("expected the stream to open, got %q", msg)
	}
}

// The token must gate the stream regardless of which transport carries it.
func TestWebSocketHandshakeRejectsAWrongToken(t *testing.T) {
	srv, _ := newTestServer(t, "the-real-token")

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if streamTokenFromRequest(r) != "the-real-token" {
			http.Error(w, "Invalid or missing session token.", http.StatusUnauthorized)
			return
		}
		upgrader := srv.streamUpgrader()
		if conn, err := upgrader.Upgrade(w, r, streamResponseHeader(r)); err == nil {
			conn.Close()
		}
	}))
	defer backend.Close()

	wsURL := "ws" + strings.TrimPrefix(backend.URL, "http")
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, http.Header{
		"Sec-WebSocket-Protocol": []string{streamTokenSubprotocolPrefix + "wrong"},
	})
	if err == nil {
		t.Fatal("expected the handshake to fail with a wrong token")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %v", resp)
	}
}
