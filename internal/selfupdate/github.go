package selfupdate

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Typed errors callers (the self-update orchestrator and the CLI) can
// switch on to produce useful user-facing messages without parsing
// strings.
var (
	ErrReleaseNotFound = errors.New("release not found")
	ErrRateLimited     = errors.New("github rate limit exceeded")
	ErrAssetNotFound   = errors.New("asset not found in release")
)

const (
	defaultBaseURL    = "https://api.github.com"
	defaultUserAgent  = "inkwell-self-update"
	defaultHTTPTimeout = 30 * time.Second
)

// GitHubClient is a minimal client for the github.com releases API,
// shaped for testability rather than completeness — only the bits the
// updater actually needs.
type GitHubClient struct {
	repo       string // "owner/repo"
	baseURL    string
	userAgent  string
	httpClient *http.Client
}

// Option configures NewGitHubClient. Existing options are baseURL,
// userAgent, and an injected http.Client (so httptest can drop in).
type Option func(*GitHubClient)

// WithBaseURL overrides the GitHub API base. Used by tests to point
// at an httptest.Server.
func WithBaseURL(url string) Option {
	return func(c *GitHubClient) { c.baseURL = url }
}

// WithHTTPClient swaps in a custom *http.Client. Used by tests; in
// production the default has a 30s timeout so a stalled API call
// can't hang the self-update flow forever.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *GitHubClient) { c.httpClient = hc }
}

// WithUserAgent overrides the User-Agent header sent with requests.
func WithUserAgent(ua string) Option {
	return func(c *GitHubClient) { c.userAgent = ua }
}

// NewGitHubClient builds a client for the named repo
// ("owner/repo"). Pass functional options to override defaults.
func NewGitHubClient(repo string, opts ...Option) *GitHubClient {
	c := &GitHubClient{
		repo:      repo,
		baseURL:   defaultBaseURL,
		userAgent: defaultUserAgent,
		httpClient: &http.Client{
			Timeout: defaultHTTPTimeout,
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Release is a trimmed view of a GitHub releases/latest payload —
// just the tag and the assets the updater needs to locate the
// tarball and checksums file.
type Release struct {
	Tag    string
	assets []asset
}

type asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

// AssetURL returns the download URL for the named asset.
func (r *Release) AssetURL(name string) (string, error) {
	for _, a := range r.assets {
		if a.Name == name {
			return a.URL, nil
		}
	}
	return "", fmt.Errorf("%w: %q", ErrAssetNotFound, name)
}

// ChecksumsURL is a convenience for the project's checksums.txt
// asset, which GoReleaser always names checksums.txt.
func (r *Release) ChecksumsURL() (string, error) {
	return r.AssetURL("checksums.txt")
}

// LatestRelease fetches /releases/latest and returns a parsed
// Release. 404 surfaces as ErrReleaseNotFound and 403 with no
// rate-limit budget remaining surfaces as ErrRateLimited so the
// caller can produce specific messaging; other statuses become
// generic "unexpected status" errors.
func (c *GitHubClient) LatestRelease() (*Release, error) {
	url := fmt.Sprintf("%s/repos/%s/releases/latest", c.baseURL, c.repo)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch latest release: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// fall through to parse
	case http.StatusNotFound:
		return nil, fmt.Errorf("%w: %s", ErrReleaseNotFound, c.repo)
	case http.StatusForbidden:
		if resp.Header.Get("X-RateLimit-Remaining") == "0" {
			return nil, fmt.Errorf("%w (try again later)", ErrRateLimited)
		}
		return nil, fmt.Errorf("forbidden fetching latest release (status 403)")
	default:
		return nil, fmt.Errorf("unexpected status fetching latest release: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	var parsed struct {
		Tag    string  `json:"tag_name"`
		Assets []asset `json:"assets"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("parse release json: %w", err)
	}

	return &Release{Tag: parsed.Tag, assets: parsed.Assets}, nil
}
