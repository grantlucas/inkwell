package inkwell

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"strconv"
	"sync"
)

// WebPreview is a Hardware implementation that serves the current display
// frame over HTTP. Useful for live browser-based preview during development.
type WebPreview struct {
	profile     *DisplayProfile
	lastCmd     byte
	captured    []byte
	mu          sync.RWMutex
	current     *image.Paletted
	encodePNG   func(w io.Writer, m image.Image) error
	subscribers map[chan struct{}]struct{}
}

var _ Hardware = (*WebPreview)(nil)

// NewWebPreview creates a WebPreview for the given display profile.
func NewWebPreview(profile *DisplayProfile) *WebPreview {
	return &WebPreview{
		profile:     profile,
		encodePNG:   png.Encode,
		subscribers: make(map[chan struct{}]struct{}),
	}
}

// Frame returns a copy of the latest display frame, or nil if no frame has
// been rendered. The returned image is safe to read and modify without
// affecting the internal state.
func (wp *WebPreview) Frame() *image.Paletted {
	wp.mu.RLock()
	defer wp.mu.RUnlock()
	if wp.current == nil {
		return nil
	}
	frame := image.NewPaletted(
		wp.current.Rect,
		append(wp.current.Palette[:0:0], wp.current.Palette...),
	)
	copy(frame.Pix, wp.current.Pix)
	return frame
}

// SendCommand tracks the last command sent. On RefreshCmd, unpacks the
// captured buffer into the current frame. Important: wp.current must only be
// replaced (not mutated in place) so that ServeHTTP can safely read the
// pointer under RLock without a data race on the underlying pixel data.
func (wp *WebPreview) SendCommand(cmd byte) error {
	if wp.profile == nil {
		return fmt.Errorf("web preview: nil display profile")
	}
	wp.mu.Lock()
	defer wp.mu.Unlock()

	wp.lastCmd = cmd
	if cmd == wp.profile.RefreshCmd && wp.captured != nil {
		if wp.profile.Color != BW {
			return fmt.Errorf("web preview: unsupported color depth %v; only BW is currently supported", wp.profile.Color)
		}
		if expected := wp.profile.BufferSize(); len(wp.captured) != expected {
			return fmt.Errorf("web preview: buffer size %d does not match expected %d", len(wp.captured), expected)
		}
		wp.current = UnpackBuffer(wp.profile, wp.captured)
		wp.notifyLocked()
	}
	return nil
}

// SendData captures the data payload when the previous command was NewBufferCmd.
func (wp *WebPreview) SendData(data []byte) error {
	if wp.profile == nil {
		return fmt.Errorf("web preview: nil display profile")
	}
	wp.mu.Lock()
	defer wp.mu.Unlock()

	if wp.lastCmd == wp.profile.NewBufferCmd {
		wp.captured = make([]byte, len(data))
		copy(wp.captured, data)
	}
	return nil
}

// ReadBusy always returns true (idle) — no real hardware to wait on.
func (wp *WebPreview) ReadBusy() bool { return true }

// Reset is a no-op for the web preview backend.
func (wp *WebPreview) Reset() error { return nil }

// Close is a no-op for the web preview backend.
func (wp *WebPreview) Close() error { return nil }

// ServeHTTP serves the current display frame as a PNG image.
// Supports an optional ?scale=N query parameter (1–10) for nearest-neighbor upscaling.
func (wp *WebPreview) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	wp.mu.RLock()
	frame := wp.current
	wp.mu.RUnlock()

	if frame == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	scale := 1
	if s := r.URL.Query().Get("scale"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > 10 {
			http.Error(w, "scale must be an integer between 1 and 10", http.StatusBadRequest)
			return
		}
		scale = n
	}

	var img image.Image = frame
	if scale > 1 {
		img = scaleImage(frame, scale)
	}

	var buf bytes.Buffer
	if err := wp.encodePNG(&buf, img); err != nil {
		http.Error(w, "failed to encode PNG", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Write(buf.Bytes())
}

// Subscribe registers a channel that receives a notification on each frame refresh.
func (wp *WebPreview) Subscribe() chan struct{} {
	wp.mu.Lock()
	defer wp.mu.Unlock()
	ch := make(chan struct{}, 1)
	wp.subscribers[ch] = struct{}{}
	return ch
}

// Unsubscribe removes a previously registered subscriber channel.
func (wp *WebPreview) Unsubscribe(ch chan struct{}) {
	wp.mu.Lock()
	defer wp.mu.Unlock()
	delete(wp.subscribers, ch)
}

// subscriberCount returns the number of active subscribers. Exported only for
// test synchronization.
func (wp *WebPreview) subscriberCount() int {
	wp.mu.RLock()
	defer wp.mu.RUnlock()
	return len(wp.subscribers)
}

// notifyLocked sends a non-blocking signal to all subscribers. Must be called
// while wp.mu is held.
func (wp *WebPreview) notifyLocked() {
	for ch := range wp.subscribers {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// Handler returns an http.Handler with routes for the preview UI, frame PNG, and SSE events.
func (wp *WebPreview) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", wp.serveHTML)
	mux.HandleFunc("GET /frame.png", wp.ServeHTTP)
	mux.HandleFunc("GET /events", wp.serveSSE)
	return mux
}

func (wp *WebPreview) serveHTML(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, htmlPage)
}

const htmlPage = `<!DOCTYPE html>
<html><head><title>Inkwell Preview</title></head>
<body style="background:#333;display:flex;justify-content:center;align-items:center;min-height:100vh;margin:0">
<img id="frame" src="/frame.png?scale=2" style="image-rendering:pixelated">
<script>
new EventSource("/events").onmessage = function() {
  document.getElementById("frame").src = "/frame.png?scale=2&t=" + Date.now();
};
</script>
</body></html>`

func (wp *WebPreview) serveSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := wp.Subscribe()
	defer wp.Unsubscribe(ch)

	for {
		select {
		case <-ch:
			fmt.Fprint(w, "data: refresh\n\n")
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

// scaleImage performs nearest-neighbor upscaling of a paletted image.
func scaleImage(src *image.Paletted, factor int) *image.Paletted {
	srcB := src.Bounds()
	dstW, dstH := srcB.Dx()*factor, srcB.Dy()*factor
	dst := image.NewPaletted(image.Rect(0, 0, dstW, dstH), src.Palette)
	for y := srcB.Min.Y; y < srcB.Max.Y; y++ {
		for x := srcB.Min.X; x < srcB.Max.X; x++ {
			ci := src.ColorIndexAt(x, y)
			if ci == 0 {
				continue // index 0 is white; dst.Pix is zero-initialized
			}
			for dy := range factor {
				for dx := range factor {
					dst.SetColorIndex(x*factor+dx, y*factor+dy, ci)
				}
			}
		}
	}
	return dst
}
