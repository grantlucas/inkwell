package inkwell

import (
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

func TestWebPreview_CapturesBufferOnDisplay(t *testing.T) {
	p := imageTestProfile()
	wp := NewWebPreview(p)

	buf := make([]byte, p.BufferSize())
	buf[0] = 0x80 // pixel (0,0) black

	sendDisplaySequence(t, wp, p, buf)

	frame := wp.Frame()
	if frame == nil {
		t.Fatal("expected frame after display sequence, got nil")
	}

	// Pixel (0,0) should be black
	r, g, b, _ := frame.At(0, 0).RGBA()
	if r != 0 || g != 0 || b != 0 {
		t.Errorf("pixel (0,0): got (%d,%d,%d), want black", r, g, b)
	}
}

// TestWebPreview_ReconstructsAfterDisplayPartialBox locks the reason
// DisplayPartialBox keeps full-screen planes: the capture/preview backend
// reconstructs the device frame from the full new plane via reconstructFrame,
// which rejects anything smaller than BufferSize(). A future change that slices
// the partial buffers to the region would break this — and this test.
func TestWebPreview_ReconstructsAfterDisplayPartialBox(t *testing.T) {
	p := imageTestProfile()
	p.PartialWindowCmd = 0x90
	p.PartialEnterCmd = 0x91
	p.PartialVCOM = []byte{0xA9, 0x07}
	p.Capabilities.PartialRefresh = true

	wp := NewWebPreview(p)
	epd := NewEPD(wp, p)

	newBuf := make([]byte, p.BufferSize())
	newBuf[0] = 0x80 // pixel (0,0) black
	prevBuf := make([]byte, p.BufferSize())

	if err := epd.DisplayPartialBox(newBuf, prevBuf, Region{X: 0, Y: 0, W: 16, H: 16}); err != nil {
		t.Fatal(err)
	}

	frame := wp.Frame()
	if frame == nil {
		t.Fatal("expected reconstructed device frame, got nil")
	}
	if r, g, b, _ := frame.At(0, 0).RGBA(); r != 0 || g != 0 || b != 0 {
		t.Errorf("pixel (0,0): got (%d,%d,%d), want black", r, g, b)
	}
}

func TestWebPreview_ServeFramePNG(t *testing.T) {
	p := imageTestProfile()
	wp := NewWebPreview(p)

	buf := make([]byte, p.BufferSize())
	sendDisplaySequence(t, wp, p, buf)

	req := httptest.NewRequest(http.MethodGet, "/frame.png", nil)
	rec := httptest.NewRecorder()
	wp.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "image/png" {
		t.Fatalf("Content-Type = %q, want %q", ct, "image/png")
	}

	img, err := png.Decode(rec.Body)
	if err != nil {
		t.Fatalf("decode PNG: %v", err)
	}
	bounds := img.Bounds()
	if bounds.Dx() != p.Width || bounds.Dy() != p.Height {
		t.Errorf("dimensions = %dx%d, want %dx%d", bounds.Dx(), bounds.Dy(), p.Width, p.Height)
	}
}

func TestWebPreview_ScaleParameter(t *testing.T) {
	p := imageTestProfile()
	wp := NewWebPreview(p)

	buf := make([]byte, p.BufferSize())
	buf[0] = 0x80 // pixel (0,0) black
	sendDisplaySequence(t, wp, p, buf)

	req := httptest.NewRequest(http.MethodGet, "/frame.png?scale=3", nil)
	rec := httptest.NewRecorder()
	wp.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	img, err := png.Decode(rec.Body)
	if err != nil {
		t.Fatalf("decode PNG: %v", err)
	}
	wantW, wantH := p.Width*3, p.Height*3
	bounds := img.Bounds()
	if bounds.Dx() != wantW || bounds.Dy() != wantH {
		t.Errorf("dimensions = %dx%d, want %dx%d", bounds.Dx(), bounds.Dy(), wantW, wantH)
	}

	// Scaled pixel (0,0) should still be black (covers 3x3 block)
	r, g, b, _ := img.At(0, 0).RGBA()
	if r != 0 || g != 0 || b != 0 {
		t.Errorf("scaled pixel (0,0): got (%d,%d,%d), want black", r, g, b)
	}
	// Pixel (3,0) should be white (next source pixel)
	r, g, b, _ = img.At(3, 0).RGBA()
	if r == 0 && g == 0 && b == 0 {
		t.Error("scaled pixel (3,0): got black, want white")
	}
}

func TestWebPreview_NoFrameReturns204(t *testing.T) {
	wp := NewWebPreview(imageTestProfile())

	if frame := wp.Frame(); frame != nil {
		t.Errorf("Frame before display: got %v, want nil", frame)
	}

	req := httptest.NewRequest(http.MethodGet, "/frame.png", nil)
	rec := httptest.NewRecorder()
	wp.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestWebPreview_NilProfileReturnsError(t *testing.T) {
	wp := NewWebPreview(nil)

	if err := wp.SendCommand(0x12); err == nil {
		t.Error("SendCommand with nil profile: expected error, got nil")
	}
	if err := wp.SendData([]byte{0x00}); err == nil {
		t.Error("SendData with nil profile: expected error, got nil")
	}
}

func TestWebPreview_BufferSizeMismatchReturnsError(t *testing.T) {
	p := imageTestProfile()
	wp := NewWebPreview(p)

	if err := wp.SendCommand(p.NewBufferCmd); err != nil {
		t.Fatalf("SendCommand(NewBufferCmd): %v", err)
	}
	if err := wp.SendData([]byte{0x00}); err != nil { // 1 byte, expected 32
		t.Fatalf("SendData: %v", err)
	}

	err := wp.SendCommand(p.RefreshCmd)
	if err == nil {
		t.Fatal("expected buffer size mismatch error, got nil")
	}
}

func TestWebPreview_InvalidScaleReturns400(t *testing.T) {
	p := imageTestProfile()
	wp := NewWebPreview(p)

	buf := make([]byte, p.BufferSize())
	sendDisplaySequence(t, wp, p, buf)

	for _, scale := range []string{"0", "-1", "11", "abc"} {
		req := httptest.NewRequest(http.MethodGet, "/frame.png?scale="+scale, nil)
		rec := httptest.NewRecorder()
		wp.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("scale=%s: status = %d, want %d", scale, rec.Code, http.StatusBadRequest)
		}
	}
}

func TestWebPreview_NonGetMethodReturns405(t *testing.T) {
	p := imageTestProfile()
	wp := NewWebPreview(p)

	buf := make([]byte, p.BufferSize())
	sendDisplaySequence(t, wp, p, buf)

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		req := httptest.NewRequest(method, "/frame.png", nil)
		rec := httptest.NewRecorder()
		wp.ServeHTTP(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s: status = %d, want %d", method, rec.Code, http.StatusMethodNotAllowed)
		}
	}
}

// gray4PreviewProfile mirrors gray4ImageProfile but is local to this
// file so the two backend test sets stay independently editable.
func gray4PreviewProfile() *DisplayProfile {
	return &DisplayProfile{
		Name:         "gray-preview",
		Width:        16,
		Height:       1,
		Color:        Gray4,
		OldBufferCmd: 0x10,
		NewBufferCmd: 0x13,
		RefreshCmd:   0x12,
	}
}

// TestWebPreview_Gray4Frame drives WebPreview with a Gray4 frame and
// verifies wp.Frame() returns the four canonical shades at the expected
// pixel positions. This is the inverse of the EPD.Display plane-split
// path: the backend captures both planes off the wire and recombines
// them into a viewable image.
func TestWebPreview_Gray4Frame(t *testing.T) {
	p := gray4PreviewProfile()
	wp := NewWebPreview(p)

	buf := []byte{0x1B, 0xE4, 0x55, 0xAA} // w,l,d,b, b,d,l,w, l*4, d*4
	sendDisplaySequence(t, wp, p, buf)

	frame := wp.Frame()
	if frame == nil {
		t.Fatal("expected Gray4 frame after display sequence, got nil")
	}

	wantY := []uint8{0xFF, 0xC0, 0x80, 0x00}
	for x, want := range wantY {
		got := color.GrayModel.Convert(frame.At(x, 0)).(color.Gray).Y
		if got != want {
			t.Errorf("pixel (%d,0) Y=0x%02X, want 0x%02X", x, got, want)
		}
	}
}

// TestWebPreview_Gray4ServesPNGWithFourShades exercises the full preview
// pipeline (capture → unpack → PNG encode → HTTP serve) for Gray4 to
// confirm a developer running color_mode=gray4 actually sees four
// distinct shades in the browser, not a 1-bit reduction.
func TestWebPreview_Gray4ServesPNGWithFourShades(t *testing.T) {
	p := gray4PreviewProfile()
	wp := NewWebPreview(p)

	buf := []byte{0x1B, 0xE4, 0x55, 0xAA}
	sendDisplaySequence(t, wp, p, buf)

	req := httptest.NewRequest(http.MethodGet, "/frame.png", nil)
	rec := httptest.NewRecorder()
	wp.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	img, err := png.Decode(rec.Body)
	if err != nil {
		t.Fatalf("decode PNG: %v", err)
	}
	seen := map[uint8]bool{}
	for x := range 16 {
		seen[color.GrayModel.Convert(img.At(x, 0)).(color.Gray).Y] = true
	}
	if len(seen) != 4 {
		t.Errorf("distinct luminances in PNG = %d, want 4 (got %v)", len(seen), seen)
	}
}

func TestWebPreview_Gray4PlaneSizeMismatch(t *testing.T) {
	p := gray4PreviewProfile()
	wp := NewWebPreview(p)

	// Capture only the new plane (skip OldBufferCmd) — reconstructFrame
	// must reject the partial state on RefreshCmd.
	if err := wp.SendCommand(p.NewBufferCmd); err != nil {
		t.Fatalf("SendCommand: %v", err)
	}
	if err := wp.SendData([]byte{0x00, 0x00}); err != nil {
		t.Fatalf("SendData: %v", err)
	}
	if err := wp.SendCommand(p.RefreshCmd); err == nil {
		t.Fatal("expected error for missing old plane, got nil")
	}
}

func TestWebPreview_EncodeErrorReturns500(t *testing.T) {
	p := imageTestProfile()
	wp := NewWebPreview(p)
	wp.encodePNG = func(w io.Writer, m image.Image) error {
		return errors.New("encode failed")
	}

	buf := make([]byte, p.BufferSize())
	sendDisplaySequence(t, wp, p, buf)

	req := httptest.NewRequest(http.MethodGet, "/frame.png", nil)
	rec := httptest.NewRecorder()
	wp.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestWebPreview_DefaultViewIsPackedDeviceBuffer(t *testing.T) {
	// The preview must default to showing the *unpacked device buffer* —
	// the dithered 1-bit representation that the e-paper panel actually
	// receives — not the high-fidelity source. The source view is opt-in
	// via ?source=1; serving source by default would mislead about how
	// the dashboard will look on hardware.
	p := imageTestProfile()
	wp := NewWebPreview(p)

	src := image.NewPaletted(image.Rect(0, 0, p.Width, p.Height), widget.PaperPalette)
	src.SetColorIndex(1, 0, widget.PaperGray50)
	wp.SetSourceFrame(src)

	// Mutate the source after handing it over — the preview must hold a copy.
	src.SetColorIndex(1, 0, widget.PaperBlack)

	// All-white packed buffer means the device sees pure white everywhere.
	buf := make([]byte, p.BufferSize())
	sendDisplaySequence(t, wp, p, buf)

	frame := wp.Frame()
	if frame == nil {
		t.Fatal("expected frame after display sequence, got nil")
	}
	r, g, b, _ := frame.At(1, 0).RGBA()
	if r != 0xFFFF || g != 0xFFFF || b != 0xFFFF {
		t.Errorf("pixel (1,0) RGBA = (0x%04X,0x%04X,0x%04X), want white (packed buffer, not gray source)", r, g, b)
	}
}

func TestWebPreview_SourceViewOptIn(t *testing.T) {
	// With ?source=1, ServeHTTP serves the high-fidelity source frame even
	// when a packed device buffer has also been captured.
	p := imageTestProfile()
	wp := NewWebPreview(p)

	src := image.NewPaletted(image.Rect(0, 0, p.Width, p.Height), widget.PaperPalette)
	src.SetColorIndex(1, 0, widget.PaperGray50)
	wp.SetSourceFrame(src)

	buf := make([]byte, p.BufferSize())
	sendDisplaySequence(t, wp, p, buf)

	req := httptest.NewRequest(http.MethodGet, "/frame.png?source=1", nil)
	rec := httptest.NewRecorder()
	wp.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	img, err := png.Decode(rec.Body)
	if err != nil {
		t.Fatalf("decode PNG: %v", err)
	}
	want := widget.PaperPalette[widget.PaperGray50].(color.Gray)
	gr, gg, gb, _ := img.At(1, 0).RGBA()
	wr, wg, wb, _ := want.RGBA()
	if gr != wr || gg != wg || gb != wb {
		t.Errorf("?source=1 pixel (1,0) RGBA = (0x%04X,0x%04X,0x%04X), want (0x%04X,0x%04X,0x%04X) PaperGray50",
			gr, gg, gb, wr, wg, wb)
	}
}

func TestWebPreview_SourceViewWithoutSourceFrameReturns204(t *testing.T) {
	// ?source=1 with no SetSourceFrame call must return 204, not the
	// packed device buffer — otherwise the toggle would silently lie
	// about which view the user is looking at.
	p := imageTestProfile()
	wp := NewWebPreview(p)

	buf := make([]byte, p.BufferSize())
	sendDisplaySequence(t, wp, p, buf) // captures a buffer, sets wp.current

	req := httptest.NewRequest(http.MethodGet, "/frame.png?source=1", nil)
	rec := httptest.NewRecorder()
	wp.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d (no source frame supplied)", rec.Code, http.StatusNoContent)
	}
}

func TestWebPreview_SetSourceFrameNilIgnored(t *testing.T) {
	wp := NewWebPreview(imageTestProfile())
	wp.SetSourceFrame(nil) // must not panic
	if wp.source != nil {
		t.Errorf("source = %v, want nil", wp.source)
	}
}

func TestWebPreview_SourceFrameAloneServesWithoutPackedBuffer(t *testing.T) {
	p := imageTestProfile()
	wp := NewWebPreview(p)

	src := image.NewPaletted(image.Rect(0, 0, p.Width, p.Height), widget.PaperPalette)
	src.SetColorIndex(0, 0, widget.PaperBlack)
	wp.SetSourceFrame(src)

	// Trigger a refresh without ever capturing a buffer.
	if err := wp.SendCommand(p.RefreshCmd); err != nil {
		t.Fatalf("SendCommand(RefreshCmd): %v", err)
	}
	frame := wp.Frame()
	if frame == nil {
		t.Fatal("expected frame from source alone, got nil")
	}
	if got := frame.ColorIndexAt(0, 0); got != widget.PaperBlack {
		t.Errorf("pixel (0,0) idx = %d, want %d (widget.PaperBlack)", got, widget.PaperBlack)
	}
}

func TestWebPreview_RefreshWithoutBufferOrSourceIsNoop(t *testing.T) {
	wp := NewWebPreview(imageTestProfile())
	if err := wp.SendCommand(wp.profile.RefreshCmd); err != nil {
		t.Fatalf("RefreshCmd with no captured/source: %v", err)
	}
	if frame := wp.Frame(); frame != nil {
		t.Errorf("Frame = %v, want nil", frame)
	}
}

func TestWebPreview_ReadBusyResetClose(t *testing.T) {
	wp := NewWebPreview(imageTestProfile())

	if !wp.ReadBusy() {
		t.Error("ReadBusy: got false, want true")
	}
	if err := wp.Reset(); err != nil {
		t.Errorf("Reset: %v", err)
	}
	if err := wp.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

// notifyLocked's send is non-blocking — when a subscriber's buffered
// channel is full, the default branch must fire and the next refresh
// must still complete. Pin the default branch by filling a
// subscriber's channel before triggering another refresh.
func TestWebPreview_NotifyDropsWhenSubscriberFull(t *testing.T) {
	profile, ok := Profiles["waveshare_7in5_v2"]
	if !ok {
		t.Fatal("missing waveshare_7in5_v2 profile")
	}
	wp := NewWebPreview(profile)

	// Prime: capture a buffer so SendCommand(RefreshCmd) calls notifyLocked.
	if err := wp.SendData(make([]byte, profile.BufferSize())); err != nil {
		t.Fatalf("SendData: %v", err)
	}

	ch := wp.Subscribe()
	defer wp.Unsubscribe(ch)

	// Fill the (cap=1) subscriber channel so the second notify hits
	// the default-branch drop.
	ch <- struct{}{}

	if err := wp.SendCommand(profile.RefreshCmd); err != nil {
		t.Fatalf("SendCommand: %v", err)
	}
	// Sanity: the subscriber should still be registered (the drop
	// doesn't remove it).
	if got := wp.subscriberCount(); got != 1 {
		t.Errorf("subscriberCount = %d, want 1", got)
	}
}
