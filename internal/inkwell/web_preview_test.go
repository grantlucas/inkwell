package inkwell

import (
	"errors"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
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

func TestWebPreview_NonBWProfileReturnsError(t *testing.T) {
	p := &DisplayProfile{
		Name:         "gray-test",
		Width:        16,
		Height:       16,
		Color:        Gray4,
		OldBufferCmd: 0x10,
		NewBufferCmd: 0x13,
		RefreshCmd:   0x12,
	}
	wp := NewWebPreview(p)

	buf := make([]byte, p.BufferSize())
	if err := wp.SendCommand(p.NewBufferCmd); err != nil {
		t.Fatalf("SendCommand(NewBufferCmd): %v", err)
	}
	if err := wp.SendData(buf); err != nil {
		t.Fatalf("SendData: %v", err)
	}

	err := wp.SendCommand(p.RefreshCmd)
	if err == nil {
		t.Fatal("expected unsupported color depth error, got nil")
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
