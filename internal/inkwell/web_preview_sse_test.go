package inkwell

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestWebPreview_SSEEventOnRefresh(t *testing.T) {
	p := imageTestProfile()
	wp := NewWebPreview(p)

	ch := wp.Subscribe()
	defer wp.Unsubscribe(ch)

	buf := make([]byte, p.BufferSize())
	sendDisplaySequence(t, wp, p, buf)

	select {
	case <-ch:
		// received notification — pass
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for SSE notification")
	}
}

func TestWebPreview_MultipleSubscribers(t *testing.T) {
	p := imageTestProfile()
	wp := NewWebPreview(p)

	ch1 := wp.Subscribe()
	ch2 := wp.Subscribe()
	defer wp.Unsubscribe(ch1)
	defer wp.Unsubscribe(ch2)

	buf := make([]byte, p.BufferSize())
	sendDisplaySequence(t, wp, p, buf)

	for i, ch := range []chan struct{}{ch1, ch2} {
		select {
		case <-ch:
			// pass
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d timed out waiting for notification", i+1)
		}
	}
}

func TestWebPreview_UnsubscribeRemovesChannel(t *testing.T) {
	p := imageTestProfile()
	wp := NewWebPreview(p)

	ch := wp.Subscribe()
	wp.Unsubscribe(ch)

	buf := make([]byte, p.BufferSize())
	sendDisplaySequence(t, wp, p, buf)

	// Channel should not receive anything after unsubscribe
	select {
	case <-ch:
		t.Fatal("received notification after unsubscribe")
	case <-time.After(50 * time.Millisecond):
		// expected — no notification
	}
}

func TestWebPreview_HTMLPageEndpoint(t *testing.T) {
	p := imageTestProfile()
	wp := NewWebPreview(p)

	mux := wp.Mux()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "EventSource") {
		t.Error("HTML page missing EventSource JS")
	}
	if !strings.Contains(body, "/frame.png") {
		t.Error("HTML page missing /frame.png reference")
	}
	if !strings.Contains(body, "/events") {
		t.Error("HTML page missing /events reference")
	}
}

func TestWebPreview_SSEEndpointHeaders(t *testing.T) {
	p := imageTestProfile()
	wp := NewWebPreview(p)

	mux := wp.Mux()

	// Use a context with cancel to end the SSE connection
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately so handler returns quickly

	req := httptest.NewRequest(http.MethodGet, "/events", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}
	cc := rec.Header().Get("Cache-Control")
	if cc != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", cc)
	}
	conn := rec.Header().Get("Connection")
	if conn != "keep-alive" {
		t.Errorf("Connection = %q, want keep-alive", conn)
	}
}

// syncRecorder wraps httptest.ResponseRecorder with a mutex for thread-safe body access.
type syncRecorder struct {
	*httptest.ResponseRecorder
	mu sync.Mutex
}

func (s *syncRecorder) Write(b []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ResponseRecorder.Write(b)
}

func (s *syncRecorder) bodyLen() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Body.Len()
}

func (s *syncRecorder) bodyString() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Body.String()
}

func TestWebPreview_SSEEndpointSendsData(t *testing.T) {
	p := imageTestProfile()
	wp := NewWebPreview(p)

	mux := wp.Mux()

	// Start SSE handler in a goroutine
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/events", nil).WithContext(ctx)
	rec := &syncRecorder{ResponseRecorder: httptest.NewRecorder()}

	done := make(chan struct{})
	go func() {
		mux.ServeHTTP(rec, req)
		close(done)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})

	// Wait for handler to subscribe
	waitFor(t, time.Second, func() bool { return wp.subscriberCount() > 0 },
		"timed out waiting for SSE handler to subscribe")

	// Trigger a refresh
	buf := make([]byte, p.BufferSize())
	sendDisplaySequence(t, wp, p, buf)

	// Wait for handler to write
	waitFor(t, time.Second, func() bool { return rec.bodyLen() > 0 },
		"timed out waiting for SSE handler to write")

	body := rec.bodyString()
	if !strings.Contains(body, "data: refresh") {
		t.Errorf("SSE body = %q, want to contain 'data: refresh'", body)
	}
}

func TestWebPreview_SSENoFlusherReturns500(t *testing.T) {
	p := imageTestProfile()
	wp := NewWebPreview(p)

	// bareWriter implements http.ResponseWriter but NOT http.Flusher.
	req := httptest.NewRequest(http.MethodGet, "/events", nil)
	rec := &bareWriter{header: http.Header{}}

	wp.serveSSE(rec, req)

	if rec.code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.code, http.StatusInternalServerError)
	}
}

// bareWriter is an http.ResponseWriter that does NOT implement http.Flusher.
type bareWriter struct {
	header http.Header
	code   int
	body   bytes.Buffer
}

func (w *bareWriter) Header() http.Header         { return w.header }
func (w *bareWriter) WriteHeader(code int)         { w.code = code }
func (w *bareWriter) Write(b []byte) (int, error)  { return w.body.Write(b) }

// waitFor polls condition until it returns true or timeout elapses.
func waitFor(t *testing.T, timeout time.Duration, condition func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal(msg)
}

func TestWebPreview_MuxRoutesFramePNG(t *testing.T) {
	p := imageTestProfile()
	wp := NewWebPreview(p)

	buf := make([]byte, p.BufferSize())
	sendDisplaySequence(t, wp, p, buf)

	mux := wp.Mux()
	req := httptest.NewRequest(http.MethodGet, "/frame.png", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "image/png" {
		t.Errorf("Content-Type = %q, want image/png", ct)
	}
}
