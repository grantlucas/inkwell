// Package fakehttp is the fake HTTP client the data tests drive the
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

// Reply is what the client answers one URL with.
type Reply struct {
	// Status is the response's status code; zero means 200.
	Status int
	Body   string
	// Err fails the request outright, as a network error would.
	Err error
	// CloseErr is returned when the response body is closed.
	CloseErr error
}

// Client answers requests from its registered replies. It satisfies
// the Do-shaped HTTPClient interfaces of both the calendar and weather
// packages, and is safe for concurrent use.
type Client struct {
	mu      sync.Mutex
	replies map[string]func(*http.Request) Reply
	counts  map[string]int
	total   int
}

// New returns a Client with no replies; every URL answers 404 until
// one is registered.
func New() *Client {
	return &Client{replies: map[string]func(*http.Request) Reply{}, counts: map[string]int{}}
}

// Serve answers url with a 200 carrying body.
func (t *Client) Serve(url, body string) { t.Set(url, Reply{Body: body}) }

// Set answers url with r, replacing any earlier reply. A url registered
// without a query string also answers that URL with any query, so a
// forecast endpoint can be served once whatever parameters are asked.
func (t *Client) Set(url string, r Reply) {
	t.Handle(url, func(*http.Request) Reply { return r })
}

// Handle answers url with whatever h builds from each request, for a
// reply that depends on what was asked, such as a forecast honouring its
// query. It matches URLs the way Set does and replaces any earlier reply.
func (t *Client) Handle(url string, h func(*http.Request) Reply) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.replies[url] = h
}

// Do answers req. A request whose context is already done fails with the
// context's error and is not counted, since it would never have reached
// upstream.
func (t *Client) Do(req *http.Request) (*http.Response, error) {
	if err := req.Context().Err(); err != nil {
		return nil, err
	}
	t.mu.Lock()
	key, h, ok := t.lookup(req)
	t.counts[key]++
	t.total++
	t.mu.Unlock()

	r := Reply{Status: http.StatusNotFound}
	if ok {
		r = h(req)
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

// lookup finds req's handler by its full URL, then by the URL without
// its query. key is what the request is counted under.
func (t *Client) lookup(req *http.Request) (key string, h func(*http.Request) Reply, ok bool) {
	key = req.URL.String()
	if h, ok = t.replies[key]; ok {
		return key, h, true
	}
	bare := *req.URL
	bare.RawQuery = ""
	if h, ok = t.replies[bare.String()]; ok {
		return bare.String(), h, true
	}
	return key, nil, false
}

// Requests reports how many requests were answered for url, as it was
// registered.
func (t *Client) Requests(url string) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.counts[url]
}

// Total reports how many requests were answered for any URL.
func (t *Client) Total() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.total
}

type body struct {
	io.Reader
	closeErr error
}

func (b body) Close() error { return b.closeErr }
