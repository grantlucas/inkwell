package fakehttp_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"sync"
	"testing"

	"github.com/grantlucas/inkwell/internal/inkwell/testutil/fakehttp"
)

func get(t *testing.T, ctx context.Context, tr *fakehttp.Client, url string) (*http.Response, error) {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	return tr.Do(req)
}

// read returns a response's status, body and close error.
func read(t *testing.T, resp *http.Response) (int, string, error) {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	return resp.StatusCode, string(body), resp.Body.Close()
}

func TestClient_Replies(t *testing.T) {
	const url = "https://cal.example/a.ics"
	boom := errors.New("boom")
	closeFail := errors.New("close failed")

	tests := []struct {
		label      string
		setup      func(*fakehttp.Client)
		ask        string
		wantErr    error
		wantStatus int
		wantBody   string
		wantClose  error
	}{
		{
			label:      "a served URL answers 200 with its body",
			setup:      func(tr *fakehttp.Client) { tr.Serve(url, "BEGIN:VCALENDAR") },
			ask:        url,
			wantStatus: http.StatusOK,
			wantBody:   "BEGIN:VCALENDAR",
		},
		{
			label:      "an unknown URL answers 404",
			setup:      func(*fakehttp.Client) {},
			ask:        url,
			wantStatus: http.StatusNotFound,
		},
		{
			label:      "a reply can set its status",
			setup:      func(tr *fakehttp.Client) { tr.Set(url, fakehttp.Reply{Status: http.StatusInternalServerError}) },
			ask:        url,
			wantStatus: http.StatusInternalServerError,
		},
		{
			label:   "a reply can fail the request",
			setup:   func(tr *fakehttp.Client) { tr.Set(url, fakehttp.Reply{Err: boom}) },
			ask:     url,
			wantErr: boom,
		},
		{
			label:      "a reply can fail closing its body",
			setup:      func(tr *fakehttp.Client) { tr.Set(url, fakehttp.Reply{Body: "x", CloseErr: closeFail}) },
			ask:        url,
			wantStatus: http.StatusOK,
			wantBody:   "x",
			wantClose:  closeFail,
		},
		{
			label: "a handler builds its reply from the request",
			setup: func(tr *fakehttp.Client) {
				tr.Handle(url, func(r *http.Request) fakehttp.Reply { return fakehttp.Reply{Body: r.URL.Query().Get("q")} })
			},
			ask:        url + "?q=echo",
			wantStatus: http.StatusOK,
			wantBody:   "echo",
		},
		{
			label:      "a URL registered without a query answers any query on it",
			setup:      func(tr *fakehttp.Client) { tr.Serve("https://api.example/v1/forecast", "{}") },
			ask:        "https://api.example/v1/forecast?latitude=43.25",
			wantStatus: http.StatusOK,
			wantBody:   "{}",
		},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			tr := fakehttp.New()
			tt.setup(tr)

			resp, err := get(t, context.Background(), tr, tt.ask)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Do: %v", err)
			}
			status, body, closeErr := read(t, resp)
			if status != tt.wantStatus || body != tt.wantBody {
				t.Errorf("got %d %q, want %d %q", status, body, tt.wantStatus, tt.wantBody)
			}
			if !errors.Is(closeErr, tt.wantClose) {
				t.Errorf("close err = %v, want %v", closeErr, tt.wantClose)
			}
		})
	}
}

// Requests are counted per registered URL and in total, from any number
// of goroutines, so a test can say how many times upstream was asked.
func TestClient_CountsRequests(t *testing.T) {
	const (
		a        = "https://cal.example/a.ics"
		b        = "https://cal.example/b.ics"
		forecast = "https://api.example/v1/forecast"
	)
	tr := fakehttp.New()
	tr.Serve(a, "")
	tr.Serve(forecast, "{}")

	var wg sync.WaitGroup
	for range 3 {
		wg.Go(func() { _, _ = get(t, context.Background(), tr, a) })
	}
	wg.Wait()
	_, _ = get(t, context.Background(), tr, b)
	_, _ = get(t, context.Background(), tr, forecast+"?days=1")
	_, _ = get(t, context.Background(), tr, forecast+"?days=8")

	for _, c := range []struct {
		url  string
		want int
	}{{a, 3}, {b, 1}, {forecast, 2}} {
		if got := tr.Requests(c.url); got != c.want {
			t.Errorf("Requests(%s) = %d, want %d", c.url, got, c.want)
		}
	}
	if got := tr.Total(); got != 6 {
		t.Errorf("Total = %d, want 6", got)
	}
}

// A request whose context is already done never reaches upstream, the way
// a real client behaves, so it fails with the context's error and is not
// counted.
func TestClient_HonoursCancelledContext(t *testing.T) {
	const url = "https://cal.example/a.ics"
	tr := fakehttp.New()
	tr.Serve(url, "")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := get(t, ctx, tr, url); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
	if got := tr.Total(); got != 0 {
		t.Errorf("Total = %d, want 0", got)
	}
}
