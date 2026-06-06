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
// Captures both wire planes (the old-buffer payload after OldBufferCmd
// and the new-buffer payload after NewBufferCmd) so it can reconstruct
// either BW or Gray4 frames — the BW path ignores the old plane, the
// Gray4 path joins both to recover the 2bpp buffer.
type WebPreview struct {
	profile      *DisplayProfile
	lastCmd      byte
	capturedOld  []byte
	capturedNew  []byte
	mu           sync.RWMutex
	current      *image.Paletted
	source       *image.Paletted // high-fidelity pre-pack frame, if supplied
	encodePNG    func(w io.Writer, m image.Image) error
	subscribers  map[chan struct{}]struct{}
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

// SendCommand tracks the last command sent. On RefreshCmd it reconstructs
// the captured device buffer into wp.current so ServeHTTP can show what
// the e-paper panel actually receives — i.e. the post-dither/quantize
// version, not the rich grayscale source. The high-fidelity source
// frame (set via SetSourceFrame) is kept on wp.source and served when
// the client opts in with ?source=1. Important: wp.current must only
// be replaced (not mutated in place) so that ServeHTTP can read the
// pointer under RLock without a data race on the underlying pixel data.
func (wp *WebPreview) SendCommand(cmd byte) error {
	if wp.profile == nil {
		return fmt.Errorf("web preview: nil display profile")
	}
	wp.mu.Lock()
	defer wp.mu.Unlock()

	wp.lastCmd = cmd
	if cmd == wp.profile.RefreshCmd {
		if wp.capturedNew != nil {
			img, err := reconstructFrame(wp.profile, wp.capturedOld, wp.capturedNew)
			if err != nil {
				return fmt.Errorf("web preview: %w", err)
			}
			wp.current = img
			wp.notifyLocked()
			return nil
		}
		// No packed buffer captured yet (e.g. tests that exercise the
		// FrameSink path in isolation). Fall back to the source frame so
		// /frame.png still returns something useful.
		if wp.source != nil {
			wp.current = wp.source
			wp.notifyLocked()
		}
	}
	return nil
}

// SetSourceFrame records the high-fidelity composited frame so it can be
// served directly by ServeHTTP. The frame is copied to decouple from any
// later compositor mutation. Implements FrameSink.
func (wp *WebPreview) SetSourceFrame(frame *image.Paletted) {
	if frame == nil {
		return
	}
	wp.mu.Lock()
	defer wp.mu.Unlock()
	dup := image.NewPaletted(
		frame.Rect,
		append(frame.Palette[:0:0], frame.Palette...),
	)
	copy(dup.Pix, frame.Pix)
	wp.source = dup
}

// SendData captures the data payload when the previous command was a
// buffer-write (OldBufferCmd or NewBufferCmd). Both planes are needed
// to reconstruct a Gray4 frame; BW reconstruction ignores the old plane.
func (wp *WebPreview) SendData(data []byte) error {
	if wp.profile == nil {
		return fmt.Errorf("web preview: nil display profile")
	}
	wp.mu.Lock()
	defer wp.mu.Unlock()

	switch wp.lastCmd {
	case wp.profile.OldBufferCmd:
		wp.capturedOld = append(wp.capturedOld[:0], data...)
	case wp.profile.NewBufferCmd:
		wp.capturedNew = append(wp.capturedNew[:0], data...)
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
// Supports two optional query parameters:
//   - scale=N (1–10): nearest-neighbor upscale for legibility.
//   - source=1: serve the pre-pack high-fidelity source frame (full grayscale,
//     for design review) instead of the unpacked device buffer (post-dither,
//     1-bit — what the panel actually shows). Default is the device view.
func (wp *WebPreview) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	wantSource := r.URL.Query().Get("source") == "1"

	wp.mu.RLock()
	var frame *image.Paletted
	if wantSource {
		frame = wp.source
	} else {
		frame = wp.current
	}
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
<body style="background:#333;color:#ddd;font:13px system-ui,sans-serif;display:flex;flex-direction:column;align-items:center;min-height:100vh;margin:0;gap:10px;padding-top:14px">
<div>
  <label><input type="radio" name="view" value="device" checked> Device (post-dither, what the e-paper panel shows)</label>
  &nbsp;&nbsp;
  <label><input type="radio" name="view" value="source"> Source (full grayscale, design intent)</label>
</div>
<img id="frame" src="/frame.png?scale=2" style="image-rendering:pixelated">
<script>
function currentSrc() {
  var v = document.querySelector('input[name=view]:checked').value;
  var q = v === 'source' ? '&source=1' : '';
  return '/frame.png?scale=2' + q + '&t=' + Date.now();
}
function refresh() { document.getElementById('frame').src = currentSrc(); }
document.querySelectorAll('input[name=view]').forEach(function(el) {
  el.addEventListener('change', refresh);
});
new EventSource('/events').onmessage = refresh;
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
