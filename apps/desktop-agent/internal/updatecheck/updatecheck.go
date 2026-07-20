// Package updatecheck reports whether a newer TabVM release is available on
// GitHub. It backs the in-app "update available" banner.
//
// Privacy / identity: TabVM is local-first and offline-friendly. The ONLY
// outbound network call this package makes is an unauthenticated GET to
// GitHub's public releases API (https://api.github.com/repos/<repo>/releases/
// latest). No credentials, cookies, or telemetry are sent. The result is cached
// for 6h and every failure (offline, rate-limited, malformed) resolves to a
// safe "no update" payload rather than an error, so the check can never block
// or break the app. Failures also refresh the cache timestamp so an offline
// host is not hammered. The frontend can disable the call entirely via
// localStorage['tabvm.updateCheck']='off'.
package updatecheck

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tabvm/desktop-agent/internal/models"
)

const (
	defaultAPIBaseURL = "https://api.github.com"
	defaultRepo       = "carauz1905/TabVM"
	defaultTTL        = 6 * time.Hour
	requestTimeout    = 5 * time.Second
	maxBodyBytes      = 1 << 20 // 1 MiB is far more than a release payload needs.
)

// Release is the subset of a GitHub release object we consume.
type Release struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

// Checker queries GitHub for the latest TabVM release and compares it against
// the running version. It caches the result and fails silently. All external
// dependencies (HTTP client, base URL, clock) are injectable for testing.
type Checker struct {
	httpClient *http.Client
	apiBaseURL string
	repo       string
	current    string
	ttl        time.Duration
	now        func() time.Time
	logger     *slog.Logger

	mu        sync.Mutex
	cached    models.UpdateStatus
	fetchedAt time.Time
	hasCache  bool
}

// Option customizes a Checker. All options are intended for wiring and tests.
type Option func(*Checker)

// WithBaseURL overrides the GitHub API base URL (used by tests).
func WithBaseURL(url string) Option {
	return func(c *Checker) { c.apiBaseURL = url }
}

// WithRepo overrides the "owner/name" repository slug.
func WithRepo(repo string) Option {
	return func(c *Checker) { c.repo = repo }
}

// WithHTTPClient overrides the HTTP client.
func WithHTTPClient(client *http.Client) Option {
	return func(c *Checker) { c.httpClient = client }
}

// WithTTL overrides the cache lifetime.
func WithTTL(ttl time.Duration) Option {
	return func(c *Checker) { c.ttl = ttl }
}

// WithNow overrides the clock (used by tests to age the cache deterministically).
func WithNow(now func() time.Time) Option {
	return func(c *Checker) { c.now = now }
}

// WithLogger wires an optional logger; without it, the checker stays silent.
func WithLogger(logger *slog.Logger) Option {
	return func(c *Checker) { c.logger = logger }
}

// WithSeededStatus pre-populates the cache so Status returns immediately without
// any network call. Used by server-level tests to stay fully offline.
func WithSeededStatus(status models.UpdateStatus, fetchedAt time.Time) Option {
	return func(c *Checker) {
		c.cached = status
		c.fetchedAt = fetchedAt
		c.hasCache = true
	}
}

// New builds a Checker for the given running version with sensible defaults.
func New(current string, opts ...Option) *Checker {
	c := &Checker{
		httpClient: &http.Client{Timeout: requestTimeout},
		apiBaseURL: defaultAPIBaseURL,
		repo:       defaultRepo,
		current:    current,
		ttl:        defaultTTL,
		now:        time.Now,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Status returns the current update status. It serves a cached result while it
// is fresh (within the TTL), otherwise it fetches from GitHub. It never returns
// an error: any failure resolves to a safe "no update" payload, which is also
// cached so offline hosts are not repeatedly probed.
func (c *Checker) Status(ctx context.Context) models.UpdateStatus {
	c.mu.Lock()
	if c.hasCache && c.now().Sub(c.fetchedAt) < c.ttl {
		cached := c.cached
		c.mu.Unlock()
		return cached
	}
	c.mu.Unlock()

	status := c.fetch(ctx)

	c.mu.Lock()
	c.cached = status
	c.fetchedAt = c.now()
	c.hasCache = true
	c.mu.Unlock()

	return status
}

// fetch performs the actual GitHub request. It returns a safe payload on any
// failure so callers never see an error.
func (c *Checker) fetch(ctx context.Context) models.UpdateStatus {
	safe := models.UpdateStatus{Current: c.current, Latest: "", UpdateAvailable: false}

	reqCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	url := strings.TrimRight(c.apiBaseURL, "/") + "/repos/" + c.repo + "/releases/latest"
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		c.debug("update check: failed to build request", "error", err)
		return safe
	}
	req.Header.Set("User-Agent", "TabVM-agent")
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.debug("update check: request failed", "error", err)
		return safe
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.debug("update check: unexpected status", "status", resp.StatusCode)
		return safe
	}

	var release Release
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBodyBytes)).Decode(&release); err != nil {
		c.debug("update check: failed to decode response", "error", err)
		return safe
	}

	tag := strings.TrimSpace(release.TagName)
	if tag == "" {
		return safe
	}

	return models.UpdateStatus{
		Current:         c.current,
		Latest:          normalizeVersion(tag),
		UpdateAvailable: isNewer(tag, c.current),
		ReleaseURL:      release.HTMLURL,
	}
}

func (c *Checker) debug(msg string, args ...any) {
	if c.logger != nil {
		c.logger.Debug(msg, args...)
	}
}

// normalizeVersion strips a leading "v" and surrounding whitespace for display.
func normalizeVersion(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}

// isNewer reports whether latest is a strictly higher release version than
// current. Both are compared as numeric major.minor.patch after stripping a
// leading "v". A pre-release (a version containing "-") is deprioritized: it is
// never considered newer than a release. Malformed input compares as false.
func isNewer(latest, current string) bool {
	l, ok := parseVersion(latest)
	if !ok {
		return false
	}
	c, ok := parseVersion(current)
	if !ok {
		return false
	}
	for i := 0; i < 3; i++ {
		if l[i] != c[i] {
			return l[i] > c[i]
		}
	}
	return false
}

// parseVersion parses "v1.2.3" / "1.2.3" into [major, minor, patch]. It returns
// ok=false for anything malformed or carrying a pre-release/build suffix (so
// pre-releases are ignored by isNewer).
func parseVersion(v string) ([3]int, bool) {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	if v == "" || strings.Contains(v, "-") {
		return [3]int{}, false
	}
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return [3]int{}, false
	}
	var out [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return [3]int{}, false
		}
		out[i] = n
	}
	return out, true
}
