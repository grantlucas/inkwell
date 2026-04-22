package datasource

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	defaultClaudeClientID   = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"
	defaultTokenEndpoint    = "https://platform.claude.com/v1/oauth/token"
	tokenExpiryBuffer       = 60 * time.Second
)

// credentialsFile is the on-disk JSON structure from the Claude CLI OAuth flow.
type credentialsFile struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresAt    string `json:"expiresAt"`
}

// tokenResponse is the JSON response from the token refresh endpoint.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

// ClaudeCredentials reads and refreshes OAuth tokens for the Claude CLI.
type ClaudeCredentials struct {
	path          string
	clientID      string
	tokenEndpoint string
	httpClient    *http.Client
	now           func() time.Time

	mu           sync.Mutex
	accessToken  string
	refreshToken string
	expiresAt    time.Time
	loaded       bool
}

// ClaudeCredentialsOption configures ClaudeCredentials.
type ClaudeCredentialsOption func(*ClaudeCredentials)

// WithHTTPClient overrides the default HTTP client.
func WithHTTPClient(c *http.Client) ClaudeCredentialsOption {
	return func(cc *ClaudeCredentials) { cc.httpClient = c }
}

// WithTokenEndpoint overrides the token refresh URL.
func WithTokenEndpoint(endpoint string) ClaudeCredentialsOption {
	return func(cc *ClaudeCredentials) { cc.tokenEndpoint = endpoint }
}

// WithCredentialsClock overrides the time source.
func WithCredentialsClock(now func() time.Time) ClaudeCredentialsOption {
	return func(cc *ClaudeCredentials) { cc.now = now }
}

// NewClaudeCredentials creates a credential reader for the given file path.
func NewClaudeCredentials(path string, opts ...ClaudeCredentialsOption) *ClaudeCredentials {
	cc := &ClaudeCredentials{
		path:          path,
		clientID:      defaultClaudeClientID,
		tokenEndpoint: defaultTokenEndpoint,
		httpClient:    http.DefaultClient,
		now:           time.Now,
	}
	for _, opt := range opts {
		opt(cc)
	}
	return cc
}

// AccessToken returns a valid access token, refreshing if expired.
func (c *ClaudeCredentials) AccessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.loaded {
		if err := c.loadFromDisk(); err != nil {
			return "", fmt.Errorf("load credentials: %w", err)
		}
	}

	if c.now().Before(c.expiresAt.Add(-tokenExpiryBuffer)) {
		return c.accessToken, nil
	}

	if err := c.refresh(ctx); err != nil {
		return "", fmt.Errorf("refresh token: %w", err)
	}
	return c.accessToken, nil
}

func (c *ClaudeCredentials) loadFromDisk() error {
	data, err := os.ReadFile(c.path)
	if err != nil {
		return err
	}

	var cf credentialsFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return fmt.Errorf("parse credentials: %w", err)
	}

	if cf.AccessToken == "" {
		return fmt.Errorf("credentials file missing access token")
	}

	expiresAt, err := time.Parse(time.RFC3339, cf.ExpiresAt)
	if err != nil {
		return fmt.Errorf("parse expires_at: %w", err)
	}

	c.accessToken = cf.AccessToken
	c.refreshToken = cf.RefreshToken
	c.expiresAt = expiresAt
	c.loaded = true
	return nil
}

func (c *ClaudeCredentials) refresh(ctx context.Context) error {
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {c.refreshToken},
		"client_id":     {c.clientID},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("token refresh returned %d: %s", resp.StatusCode, body)
	}

	var tr tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return fmt.Errorf("parse token response: %w", err)
	}

	c.accessToken = tr.AccessToken
	if tr.RefreshToken != "" {
		c.refreshToken = tr.RefreshToken
	}
	c.expiresAt = c.now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	return nil
}
