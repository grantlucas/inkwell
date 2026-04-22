package datasource

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeCredentials(t *testing.T, dir string, cf credentialsFile) string {
	t.Helper()
	path := filepath.Join(dir, "credentials.json")
	data, err := json.Marshal(cf)
	if err != nil {
		t.Fatalf("marshal credentials: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write credentials: %v", err)
	}
	return path
}

func fixedNow(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

func TestAccessToken_ValidToken(t *testing.T) {
	now := time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC)
	path := writeCredentials(t, t.TempDir(), credentialsFile{
		AccessToken:  "valid-token",
		RefreshToken: "refresh-tok",
		ExpiresAt:    now.Add(time.Hour).Format(time.RFC3339),
	})

	cc := NewClaudeCredentials(path, WithCredentialsClock(fixedNow(now)))

	tok, err := cc.AccessToken(context.Background())
	if err != nil {
		t.Fatalf("AccessToken: %v", err)
	}
	if tok != "valid-token" {
		t.Errorf("token = %q, want %q", tok, "valid-token")
	}
}

func TestAccessToken_CachesToken(t *testing.T) {
	now := time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC)
	dir := t.TempDir()
	path := writeCredentials(t, dir, credentialsFile{
		AccessToken:  "cached-token",
		RefreshToken: "refresh-tok",
		ExpiresAt:    now.Add(time.Hour).Format(time.RFC3339),
	})

	cc := NewClaudeCredentials(path, WithCredentialsClock(fixedNow(now)))

	tok1, err := cc.AccessToken(context.Background())
	if err != nil {
		t.Fatalf("first call: %v", err)
	}

	// Remove the file — second call should still work from cache.
	os.Remove(path)

	tok2, err := cc.AccessToken(context.Background())
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if tok1 != tok2 {
		t.Errorf("tokens differ: %q vs %q", tok1, tok2)
	}
}

func TestAccessToken_ExpiredRefreshes(t *testing.T) {
	now := time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/x-www-form-urlencoded" {
			t.Errorf("Content-Type = %q", ct)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm: %v", err)
		}
		if r.Form.Get("grant_type") != "refresh_token" {
			t.Errorf("grant_type = %q", r.Form.Get("grant_type"))
		}
		if r.Form.Get("refresh_token") != "my-refresh" {
			t.Errorf("refresh_token = %q", r.Form.Get("refresh_token"))
		}
		if r.Form.Get("client_id") != defaultClaudeClientID {
			t.Errorf("client_id = %q", r.Form.Get("client_id"))
		}

		json.NewEncoder(w).Encode(tokenResponse{
			AccessToken:  "new-access",
			RefreshToken: "new-refresh",
			ExpiresIn:    3600,
		})
	}))
	defer srv.Close()

	path := writeCredentials(t, t.TempDir(), credentialsFile{
		AccessToken:  "expired-token",
		RefreshToken: "my-refresh",
		ExpiresAt:    now.Add(-time.Hour).Format(time.RFC3339),
	})

	cc := NewClaudeCredentials(path,
		WithCredentialsClock(fixedNow(now)),
		WithHTTPClient(srv.Client()),
		WithTokenEndpoint(srv.URL),
	)

	tok, err := cc.AccessToken(context.Background())
	if err != nil {
		t.Fatalf("AccessToken: %v", err)
	}
	if tok != "new-access" {
		t.Errorf("token = %q, want %q", tok, "new-access")
	}
}

func TestAccessToken_RefreshWithinBuffer(t *testing.T) {
	now := time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC)

	refreshed := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		refreshed = true
		json.NewEncoder(w).Encode(tokenResponse{
			AccessToken: "refreshed",
			ExpiresIn:   3600,
		})
	}))
	defer srv.Close()

	// Token expires in 30s — within the 60s buffer.
	path := writeCredentials(t, t.TempDir(), credentialsFile{
		AccessToken:  "about-to-expire",
		RefreshToken: "refresh-tok",
		ExpiresAt:    now.Add(30 * time.Second).Format(time.RFC3339),
	})

	cc := NewClaudeCredentials(path,
		WithCredentialsClock(fixedNow(now)),
		WithHTTPClient(srv.Client()),
		WithTokenEndpoint(srv.URL),
	)

	tok, err := cc.AccessToken(context.Background())
	if err != nil {
		t.Fatalf("AccessToken: %v", err)
	}
	if !refreshed {
		t.Error("expected refresh for token within expiry buffer")
	}
	if tok != "refreshed" {
		t.Errorf("token = %q, want %q", tok, "refreshed")
	}
}

func TestAccessToken_RefreshError(t *testing.T) {
	now := time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer srv.Close()

	path := writeCredentials(t, t.TempDir(), credentialsFile{
		AccessToken:  "expired",
		RefreshToken: "bad-refresh",
		ExpiresAt:    now.Add(-time.Hour).Format(time.RFC3339),
	})

	cc := NewClaudeCredentials(path,
		WithCredentialsClock(fixedNow(now)),
		WithHTTPClient(srv.Client()),
		WithTokenEndpoint(srv.URL),
	)

	_, err := cc.AccessToken(context.Background())
	if err == nil {
		t.Fatal("expected error for failed refresh")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error = %q, want mention of 401", err.Error())
	}
}

func TestAccessToken_MissingFile(t *testing.T) {
	cc := NewClaudeCredentials("/nonexistent/path/credentials.json")
	_, err := cc.AccessToken(context.Background())
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestAccessToken_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.json")
	os.WriteFile(path, []byte("{not json"), 0o600)

	cc := NewClaudeCredentials(path)
	_, err := cc.AccessToken(context.Background())
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestAccessToken_MissingAccessToken(t *testing.T) {
	path := writeCredentials(t, t.TempDir(), credentialsFile{
		RefreshToken: "refresh-tok",
		ExpiresAt:    time.Now().Add(time.Hour).Format(time.RFC3339),
	})

	cc := NewClaudeCredentials(path)
	_, err := cc.AccessToken(context.Background())
	if err == nil {
		t.Fatal("expected error for missing access token")
	}
	if !strings.Contains(err.Error(), "missing access token") {
		t.Errorf("error = %q, want mention of missing access token", err.Error())
	}
}

func TestAccessToken_InvalidExpiresAt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.json")
	data := `{"accessToken":"tok","refreshToken":"ref","expiresAt":"not-a-date"}`
	os.WriteFile(path, []byte(data), 0o600)

	cc := NewClaudeCredentials(path)
	_, err := cc.AccessToken(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid expiresAt")
	}
}

func TestAccessToken_RefreshBadJSON(t *testing.T) {
	now := time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("{bad json"))
	}))
	defer srv.Close()

	path := writeCredentials(t, t.TempDir(), credentialsFile{
		AccessToken:  "expired",
		RefreshToken: "refresh-tok",
		ExpiresAt:    now.Add(-time.Hour).Format(time.RFC3339),
	})

	cc := NewClaudeCredentials(path,
		WithCredentialsClock(fixedNow(now)),
		WithHTTPClient(srv.Client()),
		WithTokenEndpoint(srv.URL),
	)

	_, err := cc.AccessToken(context.Background())
	if err == nil {
		t.Fatal("expected error for bad token response JSON")
	}
}

func TestAccessToken_RefreshInvalidEndpoint(t *testing.T) {
	now := time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC)

	path := writeCredentials(t, t.TempDir(), credentialsFile{
		AccessToken:  "expired",
		RefreshToken: "refresh-tok",
		ExpiresAt:    now.Add(-time.Hour).Format(time.RFC3339),
	})

	cc := NewClaudeCredentials(path,
		WithCredentialsClock(fixedNow(now)),
		WithTokenEndpoint("://invalid-url"),
	)

	_, err := cc.AccessToken(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid token endpoint URL")
	}
}

func TestAccessToken_RefreshHTTPError(t *testing.T) {
	now := time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	srv.Close() // Close immediately to cause connection error.

	path := writeCredentials(t, t.TempDir(), credentialsFile{
		AccessToken:  "expired",
		RefreshToken: "refresh-tok",
		ExpiresAt:    now.Add(-time.Hour).Format(time.RFC3339),
	})

	cc := NewClaudeCredentials(path,
		WithCredentialsClock(fixedNow(now)),
		WithHTTPClient(srv.Client()),
		WithTokenEndpoint(srv.URL),
	)

	_, err := cc.AccessToken(context.Background())
	if err == nil {
		t.Fatal("expected error for HTTP connection failure")
	}
}

func TestAccessToken_RefreshKeepsOldRefreshToken(t *testing.T) {
	now := time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Response without refresh_token — should keep old one.
		json.NewEncoder(w).Encode(tokenResponse{
			AccessToken: "new-access",
			ExpiresIn:   3600,
		})
	}))
	defer srv.Close()

	path := writeCredentials(t, t.TempDir(), credentialsFile{
		AccessToken:  "expired",
		RefreshToken: "original-refresh",
		ExpiresAt:    now.Add(-time.Hour).Format(time.RFC3339),
	})

	cc := NewClaudeCredentials(path,
		WithCredentialsClock(fixedNow(now)),
		WithHTTPClient(srv.Client()),
		WithTokenEndpoint(srv.URL),
	)

	tok, err := cc.AccessToken(context.Background())
	if err != nil {
		t.Fatalf("AccessToken: %v", err)
	}
	if tok != "new-access" {
		t.Errorf("token = %q, want %q", tok, "new-access")
	}
	// The refresh token should still be the original.
	if cc.refreshToken != "original-refresh" {
		t.Errorf("refreshToken = %q, want %q", cc.refreshToken, "original-refresh")
	}
}
