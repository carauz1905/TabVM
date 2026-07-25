package server

import (
	"io"
	"net/http"
	"net/http/httptest"
)

// testHost is the loopback authority every test request carries.
//
// The agent refuses any request whose Host header is not a loopback name (see
// withHostCheck), which is what stops a DNS-rebinding attacker from reaching the
// API. httptest.NewRequest defaults the Host to "example.com" when the target is
// a path rather than an absolute URL, so a test request that does not set it
// explicitly would be rejected before it ever reaches a handler.
const testHost = "127.0.0.1:5230"

// newTestRequest builds an incoming server request the way the local browser
// sends one. Prefer it over httptest.NewRequest directly so the Host header is
// always set; a test that deliberately exercises a foreign Host should call
// httptest.NewRequest and set req.Host itself.
func newTestRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	req.Host = testHost
	return req
}
