// Package fakehttp is the fake HTTP transport the data tests drive the
// calendar and weather modules through. It is the only adapter those
// tests swap: everything behind the HTTP client runs for real, so
// fetching, parsing, caching and windowing are tested together the way
// they run.
//
// It serves canned replies by URL and counts the requests it answers,
// so a test can say how many times upstream was asked.
package fakehttp

import (
	"io"
	"net/http"
	"strings"
	"sync"
)

// Reply is what the transport answers one URL with.
type Reply struct {
	// Status is the response's status code; zero means 200.
	Status int
	Body   string
	// Err fails the request outright, as a network error would.
	Err error
	// CloseErr is returned when the response body is closed.
	CloseErr error
}

// Transport answers requests from its registered replies. It satisfies
// the Do-shaped HTTPClient interfaces of both the calendar and weather
// packages, and is safe for concurrent use.
type Transport struct {
	mu      sync.Mutex
	replies map[string]Reply
	counts  map[string]int
	total   int
}

// New returns a Transport with no replies; every URL answers 404 until
// one is registered.
func New() *Transport {
	return &Transport{replies: map[string]Reply{}, counts: map[string]int{}}
}

// Serve answers url with a 200 carrying body.
func (t *Transport) Serve(url, body string) { t.Set(url, Reply{Body: body}) }

// Set answers url with r, replacing any earlier reply. A url registered
// without a query string also answers that URL with any query, so a
// forecast endpoint can be served once whatever parameters are asked.
func (t *Transport) Set(url string, r Reply) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.replies[url] = r
}

// Do answers req. A request whose context is already done fails with the
// context's error and is not counted, since it would never have reached
// upstream.
func (t *Transport) Do(req *http.Request) (*http.Response, error) {
	if err := req.Context().Err(); err != nil {
		return nil, err
	}
	t.mu.Lock()
	key, r, ok := t.lookup(req)
	t.counts[key]++
	t.total++
	t.mu.Unlock()

	if !ok {
		r = Reply{Status: http.StatusNotFound}
	}
	if r.Err != nil {
		return nil, r.Err
	}
	status := r.Status
	if status == 0 {
		status = http.StatusOK
	}
	return &http.Response{
		StatusCode: status,
		Body:       body{Reader: strings.NewReader(r.Body), closeErr: r.CloseErr},
		Request:    req,
	}, nil
}

// lookup finds req's reply by its full URL, then by the URL without its
// query. key is what the request is counted under.
func (t *Transport) lookup(req *http.Request) (key string, r Reply, ok bool) {
	key = req.URL.String()
	if r, ok = t.replies[key]; ok {
		return key, r, true
	}
	bare := *req.URL
	bare.RawQuery = ""
	if r, ok = t.replies[bare.String()]; ok {
		return bare.String(), r, true
	}
	return key, Reply{}, false
}

// Requests reports how many requests were answered for url, as it was
// registered.
func (t *Transport) Requests(url string) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.counts[url]
}

// Total reports how many requests were answered for any URL.
func (t *Transport) Total() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.total
}

type body struct {
	io.Reader
	closeErr error
}

func (b body) Close() error { return b.closeErr }
