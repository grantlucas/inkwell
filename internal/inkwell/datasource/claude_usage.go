package datasource

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

const defaultUsageAPIURL = "https://api.anthropic.com/api/oauth/usage"

// Compile-time interface check.
var _ widget.UsageSource = (*ClaudeUsageClient)(nil)

// TokenSource provides an access token for API calls.
type TokenSource interface {
	AccessToken(ctx context.Context) (string, error)
}

type usageWindow struct {
	Utilization float64 `json:"utilization"`
	ResetsAt    string  `json:"resets_at"`
}

type usageResponse struct {
	FiveHour usageWindow `json:"five_hour"`
	SevenDay usageWindow `json:"seven_day"`
}

// ClaudeUsageClient fetches Claude API usage data.
type ClaudeUsageClient struct {
	tokenSource TokenSource
	httpClient  *http.Client
	apiURL      string
}

// ClaudeUsageOption configures ClaudeUsageClient.
type ClaudeUsageOption func(*ClaudeUsageClient)

// WithUsageHTTPClient overrides the default HTTP client.
func WithUsageHTTPClient(c *http.Client) ClaudeUsageOption {
	return func(cu *ClaudeUsageClient) { cu.httpClient = c }
}

// WithUsageAPIURL overrides the usage API endpoint URL.
func WithUsageAPIURL(url string) ClaudeUsageOption {
	return func(cu *ClaudeUsageClient) { cu.apiURL = url }
}

// NewClaudeUsageClient creates a client that fetches Claude usage data.
func NewClaudeUsageClient(ts TokenSource, opts ...ClaudeUsageOption) *ClaudeUsageClient {
	cu := &ClaudeUsageClient{
		tokenSource: ts,
		httpClient:  http.DefaultClient,
		apiURL:      defaultUsageAPIURL,
	}
	for _, opt := range opts {
		opt(cu)
	}
	return cu
}

// Usage fetches the current Claude API usage snapshot.
func (c *ClaudeUsageClient) Usage(ctx context.Context) (widget.UsageSnapshot, error) {
	token, err := c.tokenSource.AccessToken(ctx)
	if err != nil {
		return widget.UsageSnapshot{}, fmt.Errorf("get access token: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiURL, nil)
	if err != nil {
		return widget.UsageSnapshot{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return widget.UsageSnapshot{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return widget.UsageSnapshot{}, fmt.Errorf("usage API returned %d: %s", resp.StatusCode, body)
	}

	var ur usageResponse
	if err := json.NewDecoder(resp.Body).Decode(&ur); err != nil {
		return widget.UsageSnapshot{}, fmt.Errorf("parse usage response: %w", err)
	}

	fiveHourResets, err := time.Parse(time.RFC3339, ur.FiveHour.ResetsAt)
	if err != nil {
		return widget.UsageSnapshot{}, fmt.Errorf("parse five_hour.resets_at: %w", err)
	}

	sevenDayResets, err := time.Parse(time.RFC3339, ur.SevenDay.ResetsAt)
	if err != nil {
		return widget.UsageSnapshot{}, fmt.Errorf("parse seven_day.resets_at: %w", err)
	}

	return widget.UsageSnapshot{
		FiveHourUtilization: ur.FiveHour.Utilization,
		FiveHourResetsAt:    fiveHourResets,
		SevenDayUtilization: ur.SevenDay.Utilization,
		SevenDayResetsAt:    sevenDayResets,
	}, nil
}
