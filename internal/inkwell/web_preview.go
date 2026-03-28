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
	profile   *DisplayProfile
	lastCmd   byte
	captured  []byte
	mu        sync.RWMutex
	current   *image.Paletted
	encodePNG func(w io.Writer, m image.Image) error
}

var _ Hardware = (*WebPreview)(nil)

// NewWebPreview creates a WebPreview for the given display profile.
func NewWebPreview(profile *DisplayProfile) *WebPreview {
	return &WebPreview{profile: profile, encodePNG: png.Encode}
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
			for dy := 0; dy < factor; dy++ {
				for dx := 0; dx < factor; dx++ {
					dst.SetColorIndex(x*factor+dx, y*factor+dy, ci)
				}
			}
		}
	}
	return dst
}
