package selfupdate

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fixtureLatest is a representative releases/latest payload trimmed
// to what the client actually consumes.
const fixtureLatest = `{
  "tag_name": "v1.2.3",
  "assets": [
    {"name": "inkwell-linux-arm64.tar.gz",  "browser_download_url": "https://example.com/arm64.tar.gz"},
    {"name": "inkwell-linux-armv7.tar.gz",  "browser_download_url": "https://example.com/armv7.tar.gz"},
    {"name": "checksums.txt",               "browser_download_url": "https://example.com/checksums.txt"}
  ]
}`

func TestGitHubClient_LatestSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Errorf("missing User-Agent header")
		}
		if !strings.HasSuffix(r.URL.Path, "/releases/latest") {
			t.Errorf("path = %q, want suffix /releases/latest", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fixtureLatest))
	}))
	defer srv.Close()

	c := NewGitHubClient("grantlucas/inkwell", WithBaseURL(srv.URL), WithHTTPClient(srv.Client()))
	rel, err := c.LatestRelease()
	if err != nil {
		t.Fatalf("LatestRelease: %v", err)
	}
	if rel.Tag != "v1.2.3" {
		t.Errorf("Tag = %q, want v1.2.3", rel.Tag)
	}
	got, err := rel.AssetURL("inkwell-linux-arm64.tar.gz")
	if err != nil {
		t.Fatalf("AssetURL: %v", err)
	}
	if got != "https://example.com/arm64.tar.gz" {
		t.Errorf("AssetURL = %q, want arm64.tar.gz URL", got)
	}
	csum, err := rel.ChecksumsURL()
	if err != nil {
		t.Fatalf("ChecksumsURL: %v", err)
	}
	if csum != "https://example.com/checksums.txt" {
		t.Errorf("ChecksumsURL = %q, want checksums URL", csum)
	}

	if _, err := rel.AssetURL("nonexistent.tar.gz"); !errors.Is(err, ErrAssetNotFound) {
		t.Errorf("AssetURL(nonexistent) error = %v, want ErrAssetNotFound", err)
	}
}

func TestGitHubClient_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Not Found"}`))
	}))
	defer srv.Close()

	c := NewGitHubClient("missing/repo", WithBaseURL(srv.URL), WithHTTPClient(srv.Client()))
	_, err := c.LatestRelease()
	if !errors.Is(err, ErrReleaseNotFound) {
		t.Errorf("err = %v, want ErrReleaseNotFound", err)
	}
}

func TestGitHubClient_RateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"rate limit exceeded"}`))
	}))
	defer srv.Close()

	c := NewGitHubClient("any/repo", WithBaseURL(srv.URL), WithHTTPClient(srv.Client()))
	_, err := c.LatestRelease()
	if !errors.Is(err, ErrRateLimited) {
		t.Errorf("err = %v, want ErrRateLimited", err)
	}
}

func TestGitHubClient_NetworkFailure(t *testing.T) {
	// Server that closes the connection immediately — surfaces as a
	// transport-layer error in the client.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Fatal("hijacker not supported")
		}
		conn, _, _ := hj.Hijack()
		conn.Close()
	}))
	defer srv.Close()

	c := NewGitHubClient("any/repo", WithBaseURL(srv.URL), WithHTTPClient(srv.Client()))
	_, err := c.LatestRelease()
	if err == nil {
		t.Fatal("expected network error, got nil")
	}
	if errors.Is(err, ErrReleaseNotFound) || errors.Is(err, ErrRateLimited) {
		t.Errorf("network failure wrongly classified as typed error: %v", err)
	}
}

func TestGitHubClient_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{not valid json`))
	}))
	defer srv.Close()

	c := NewGitHubClient("any/repo", WithBaseURL(srv.URL), WithHTTPClient(srv.Client()))
	_, err := c.LatestRelease()
	if err == nil {
		t.Fatal("expected json error, got nil")
	}
}

func TestGitHubClient_DefaultBaseURL(t *testing.T) {
	c := NewGitHubClient("grantlucas/inkwell")
	if !strings.HasPrefix(c.baseURL, "https://api.github.com") {
		t.Errorf("baseURL = %q, want default api.github.com", c.baseURL)
	}
	if c.httpClient.Timeout == 0 {
		t.Errorf("default HTTP client should have a non-zero timeout")
	}
}

func TestGitHubClient_OtherErrorStatus(t *testing.T) {
	// 500 is neither 404 nor rate-limit — should still error, but
	// without claiming one of the typed sentinels.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewGitHubClient("any/repo", WithBaseURL(srv.URL), WithHTTPClient(srv.Client()))
	_, err := c.LatestRelease()
	if err == nil {
		t.Fatal("expected error for 500")
	}
	if errors.Is(err, ErrReleaseNotFound) || errors.Is(err, ErrRateLimited) {
		t.Errorf("500 wrongly classified: %v", err)
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error message should include status code: %v", err)
	}
}

// TestGitHubClient_DefaultClientTimeout exists to lock in that the
// default client doesn't hang indefinitely on a stalled server —
// otherwise self-update would block a Pi on a flaky network instead
// of failing fast.
func TestGitHubClient_DefaultClientTimeout(t *testing.T) {
	c := NewGitHubClient("grantlucas/inkwell")
	if c.httpClient.Timeout > 60*time.Second {
		t.Errorf("default timeout = %v, want <= 60s", c.httpClient.Timeout)
	}
}

// TestGitHubClient_WithHTTPClientNil confirms WithHTTPClient(nil) is
// a no-op: the constructor's default client stays in place rather
// than getting nilled out (which would panic on the next Do call).
func TestGitHubClient_WithHTTPClientNil(t *testing.T) {
	c := NewGitHubClient("any/repo", WithHTTPClient(nil))
	if c.httpClient == nil {
		t.Fatal("WithHTTPClient(nil) must not nil out the client")
	}
	if c.httpClient.Timeout == 0 {
		t.Errorf("default timeout should be preserved when WithHTTPClient(nil)")
	}
}

func TestGitHubClient_WithUserAgent(t *testing.T) {
	gotUA := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte(fixtureLatest))
	}))
	defer srv.Close()

	c := NewGitHubClient("any/repo",
		WithBaseURL(srv.URL),
		WithHTTPClient(srv.Client()),
		WithUserAgent("custom-agent/1.0"),
	)
	if _, err := c.LatestRelease(); err != nil {
		t.Fatalf("LatestRelease: %v", err)
	}
	if gotUA != "custom-agent/1.0" {
		t.Errorf("UA = %q, want %q", gotUA, "custom-agent/1.0")
	}
}

// TestGitHubClient_ForbiddenNotRateLimited covers a 403 that isn't a
// rate-limit (X-RateLimit-Remaining is nonzero or absent) — caller
// gets a generic forbidden error, not ErrRateLimited.
func TestGitHubClient_ForbiddenNotRateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Intentionally no X-RateLimit-Remaining header.
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	c := NewGitHubClient("any/repo", WithBaseURL(srv.URL), WithHTTPClient(srv.Client()))
	_, err := c.LatestRelease()
	if err == nil {
		t.Fatal("expected error")
	}
	if errors.Is(err, ErrRateLimited) {
		t.Errorf("403 without rate-limit headers should not surface as ErrRateLimited, got %v", err)
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("error should mention 403: %v", err)
	}
}

// TestGitHubClient_InvalidBaseURL covers the http.NewRequest error
// branch — an unparseable URL fails before any network call.
func TestGitHubClient_InvalidBaseURL(t *testing.T) {
	// A control character in the URL makes http.NewRequest reject it.
	c := NewGitHubClient("any/repo", WithBaseURL("http://invalid\x00host"))
	_, err := c.LatestRelease()
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
	if !strings.Contains(err.Error(), "build request") {
		t.Errorf("expected \"build request\" in error: %v", err)
	}
}

// errorBody is an io.ReadCloser that fails on the first Read,
// covering the io.ReadAll error path in LatestRelease.
type errorBody struct{}

func (errorBody) Read([]byte) (int, error) { return 0, fmt.Errorf("simulated read error") }
func (errorBody) Close() error             { return nil }

type fakeTransport struct{ resp *http.Response }

func (f fakeTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return f.resp, nil
}

func TestGitHubClient_BodyReadError(t *testing.T) {
	rt := fakeTransport{resp: &http.Response{
		StatusCode: http.StatusOK,
		Body:       errorBody{},
		Header:     make(http.Header),
	}}
	c := NewGitHubClient("any/repo", WithHTTPClient(&http.Client{Transport: rt}))
	_, err := c.LatestRelease()
	if err == nil {
		t.Fatal("expected read error")
	}
	if !strings.Contains(err.Error(), "read response body") {
		t.Errorf("expected \"read response body\" in error: %v", err)
	}
}

var _ io.ReadCloser = errorBody{}
