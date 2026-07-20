package updatecheck

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestIsNewer(t *testing.T) {
	cases := []struct {
		name    string
		latest  string
		current string
		want    bool
	}{
		{"newer patch", "v0.1.3", "0.1.2", true},
		{"newer minor", "0.2.0", "0.1.9", true},
		{"newer major", "1.0.0", "0.9.9", true},
		{"equal", "v0.1.2", "0.1.2", false},
		{"equal without v", "0.1.2", "0.1.2", false},
		{"older", "v0.1.1", "0.1.2", false},
		{"older major", "0.9.0", "1.0.0", false},
		{"prerelease is not newer", "v0.2.0-rc1", "0.1.2", false},
		{"prerelease build metadata", "0.2.0-beta.1", "0.1.2", false},
		{"malformed latest", "garbage", "0.1.2", false},
		{"empty latest", "", "0.1.2", false},
		{"non-numeric segment", "v0.a.3", "0.1.2", false},
		{"too few segments", "v1.2", "0.1.2", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isNewer(tc.latest, tc.current); got != tc.want {
				t.Fatalf("isNewer(%q, %q) = %v, want %v", tc.latest, tc.current, got, tc.want)
			}
		})
	}
}

// newTestChecker builds a Checker pointed at the given test server URL with a
// controllable clock, so no test performs real network I/O.
func newTestChecker(baseURL, current string, now func() time.Time) *Checker {
	return New(current,
		WithBaseURL(baseURL),
		WithNow(now),
		WithHTTPClient(&http.Client{Timeout: 2 * time.Second}),
	)
}

func TestStatusReportsNewerRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); got != "TabVM-agent" {
			t.Errorf("expected User-Agent TabVM-agent, got %q", got)
		}
		if got := r.Header.Get("Accept"); got != "application/vnd.github+json" {
			t.Errorf("expected GitHub Accept header, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v0.1.3","html_url":"https://github.com/carauz1905/TabVM/releases/tag/v0.1.3"}`))
	}))
	defer server.Close()

	checker := newTestChecker(server.URL, "0.1.2", time.Now)
	status := checker.Status(context.Background())

	if !status.UpdateAvailable {
		t.Fatalf("expected UpdateAvailable=true, got %+v", status)
	}
	if status.Latest != "0.1.3" {
		t.Fatalf("expected normalized Latest 0.1.3, got %q", status.Latest)
	}
	if status.Current != "0.1.2" {
		t.Fatalf("expected Current 0.1.2, got %q", status.Current)
	}
	if status.ReleaseURL != "https://github.com/carauz1905/TabVM/releases/tag/v0.1.3" {
		t.Fatalf("expected release URL, got %q", status.ReleaseURL)
	}
}

func TestStatusEqualVersionNoUpdate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"v0.1.2","html_url":"https://example.com"}`))
	}))
	defer server.Close()

	status := newTestChecker(server.URL, "0.1.2", time.Now).Status(context.Background())
	if status.UpdateAvailable {
		t.Fatalf("expected no update for equal version, got %+v", status)
	}
}

func TestStatusOlderVersionNoUpdate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"v0.1.1","html_url":"https://example.com"}`))
	}))
	defer server.Close()

	status := newTestChecker(server.URL, "0.1.2", time.Now).Status(context.Background())
	if status.UpdateAvailable {
		t.Fatalf("expected no update for older version, got %+v", status)
	}
}

func TestStatusServerErrorReturnsSafeResult(t *testing.T) {
	for _, code := range []int{http.StatusInternalServerError, http.StatusNotFound} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(code)
		}))

		status := newTestChecker(server.URL, "0.1.2", time.Now).Status(context.Background())
		if status.UpdateAvailable {
			t.Fatalf("expected safe result for HTTP %d, got %+v", code, status)
		}
		if status.Current != "0.1.2" {
			t.Fatalf("expected Current preserved on failure, got %q", status.Current)
		}
		if status.Latest != "" {
			t.Fatalf("expected empty Latest on failure, got %q", status.Latest)
		}
		server.Close()
	}
}

func TestStatusEmptyTagReturnsSafeResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"","html_url":"https://example.com"}`))
	}))
	defer server.Close()

	status := newTestChecker(server.URL, "0.1.2", time.Now).Status(context.Background())
	if status.UpdateAvailable || status.Latest != "" {
		t.Fatalf("expected safe result for empty tag, got %+v", status)
	}
}

func TestStatusCachesWithinTTL(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		_, _ = w.Write([]byte(`{"tag_name":"v0.1.3","html_url":"https://example.com"}`))
	}))
	defer server.Close()

	clock := time.Now()
	nowFn := func() time.Time { return clock }
	checker := newTestChecker(server.URL, "0.1.2", nowFn)

	first := checker.Status(context.Background())
	if !first.UpdateAvailable {
		t.Fatalf("expected first call to report an update, got %+v", first)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected 1 server call after first Status, got %d", got)
	}

	// A second call within the TTL must be served from cache.
	checker.Status(context.Background())
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected the cache to serve the second call, got %d server calls", got)
	}

	// Advancing the clock past the TTL forces a refetch.
	clock = clock.Add(7 * time.Hour)
	checker.Status(context.Background())
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("expected a refetch after the TTL expired, got %d server calls", got)
	}
}

func TestStatusThrottlesFailures(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	clock := time.Now()
	checker := newTestChecker(server.URL, "0.1.2", func() time.Time { return clock })

	checker.Status(context.Background())
	checker.Status(context.Background())
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected failures to be throttled within the TTL, got %d server calls", got)
	}
}
