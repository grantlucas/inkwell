package datasource

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

type stubTokenSource struct {
	token string
	err   error
}

func (s *stubTokenSource) AccessToken(_ context.Context) (string, error) {
	return s.token, s.err
}

func usageServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *ClaudeUsageClient) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	client := NewClaudeUsageClient(
		&stubTokenSource{token: "test-token"},
		WithUsageHTTPClient(srv.Client()),
		WithUsageAPIURL(srv.URL),
	)
	return srv, client
}

func TestUsage_Success(t *testing.T) {
	_, client := usageServer(t, func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(usageResponse{
			FiveHour: usageWindow{
				Utilization: 0.42,
				ResetsAt:    "2026-04-22T03:00:00Z",
			},
			SevenDay: usageWindow{
				Utilization: 0.15,
				ResetsAt:    "2026-04-28T00:00:00Z",
			},
		})
	})

	snap, err := client.Usage(context.Background())
	if err != nil {
		t.Fatalf("Usage: %v", err)
	}

	if snap.FiveHourUtilization != 0.42 {
		t.Errorf("FiveHourUtilization = %f, want 0.42", snap.FiveHourUtilization)
	}
	if snap.SevenDayUtilization != 0.15 {
		t.Errorf("SevenDayUtilization = %f, want 0.15", snap.SevenDayUtilization)
	}

	wantFiveHour := time.Date(2026, 4, 22, 3, 0, 0, 0, time.UTC)
	if !snap.FiveHourResetsAt.Equal(wantFiveHour) {
		t.Errorf("FiveHourResetsAt = %v, want %v", snap.FiveHourResetsAt, wantFiveHour)
	}

	wantSevenDay := time.Date(2026, 4, 28, 0, 0, 0, 0, time.UTC)
	if !snap.SevenDayResetsAt.Equal(wantSevenDay) {
		t.Errorf("SevenDayResetsAt = %v, want %v", snap.SevenDayResetsAt, wantSevenDay)
	}
}

func TestUsage_SetsHeaders(t *testing.T) {
	_, client := usageServer(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q, want %q", got, "Bearer test-token")
		}
		if got := r.Header.Get("anthropic-beta"); got != "oauth-2025-04-20" {
			t.Errorf("anthropic-beta = %q, want %q", got, "oauth-2025-04-20")
		}
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		json.NewEncoder(w).Encode(usageResponse{
			FiveHour: usageWindow{Utilization: 0, ResetsAt: "2026-04-22T00:00:00Z"},
			SevenDay: usageWindow{Utilization: 0, ResetsAt: "2026-04-22T00:00:00Z"},
		})
	})

	if _, err := client.Usage(context.Background()); err != nil {
		t.Fatalf("Usage: %v", err)
	}
}

func TestUsage_TokenError(t *testing.T) {
	client := NewClaudeUsageClient(&stubTokenSource{err: errors.New("no token")})

	_, err := client.Usage(context.Background())
	if err == nil {
		t.Fatal("expected error when token source fails")
	}
	if !strings.Contains(err.Error(), "no token") {
		t.Errorf("error = %q, want mention of token error", err.Error())
	}
}

func TestUsage_Non200(t *testing.T) {
	_, client := usageServer(t, func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	})

	_, err := client.Usage(context.Background())
	if err == nil {
		t.Fatal("expected error for non-200 response")
	}
	if !strings.Contains(err.Error(), "429") {
		t.Errorf("error = %q, want mention of 429", err.Error())
	}
}

func TestUsage_MalformedJSON(t *testing.T) {
	_, client := usageServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("{bad json"))
	})

	_, err := client.Usage(context.Background())
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestUsage_InvalidFiveHourResetsAt(t *testing.T) {
	_, client := usageServer(t, func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(usageResponse{
			FiveHour: usageWindow{Utilization: 0.5, ResetsAt: "not-a-date"},
			SevenDay: usageWindow{Utilization: 0.1, ResetsAt: "2026-04-28T00:00:00Z"},
		})
	})

	_, err := client.Usage(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid five_hour.resets_at")
	}
	if !strings.Contains(err.Error(), "five_hour") {
		t.Errorf("error = %q, want mention of five_hour", err.Error())
	}
}

func TestUsage_InvalidSevenDayResetsAt(t *testing.T) {
	_, client := usageServer(t, func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(usageResponse{
			FiveHour: usageWindow{Utilization: 0.5, ResetsAt: "2026-04-22T03:00:00Z"},
			SevenDay: usageWindow{Utilization: 0.1, ResetsAt: "not-a-date"},
		})
	})

	_, err := client.Usage(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid seven_day.resets_at")
	}
	if !strings.Contains(err.Error(), "seven_day") {
		t.Errorf("error = %q, want mention of seven_day", err.Error())
	}
}

func TestUsage_InvalidAPIURL(t *testing.T) {
	client := NewClaudeUsageClient(
		&stubTokenSource{token: "tok"},
		WithUsageAPIURL("://invalid"),
	)

	_, err := client.Usage(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid API URL")
	}
}

func TestUsage_HTTPClientError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	srv.Close()

	client := NewClaudeUsageClient(
		&stubTokenSource{token: "tok"},
		WithUsageHTTPClient(srv.Client()),
		WithUsageAPIURL(srv.URL),
	)

	_, err := client.Usage(context.Background())
	if err == nil {
		t.Fatal("expected error for closed server")
	}
}

func TestUsage_ReturnsZeroSnapshotOnError(t *testing.T) {
	client := NewClaudeUsageClient(&stubTokenSource{err: errors.New("fail")})

	snap, _ := client.Usage(context.Background())
	if snap != (widget.UsageSnapshot{}) {
		t.Errorf("expected zero UsageSnapshot on error, got %+v", snap)
	}
}
