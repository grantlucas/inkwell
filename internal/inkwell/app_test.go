package inkwell

import (
	"context"
	"fmt"
	"image"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestNewApp_ValidConfig(t *testing.T) {
	cfg, err := LoadConfig(strings.NewReader(`
display: waveshare_7in5_v2
backend: preview
`))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	mock := &MockHardware{}
	app, err := NewApp(cfg, WithHardware(mock), WithInterval(time.Millisecond))
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}

	if app == nil {
		t.Fatal("expected non-nil App")
	}
}

func TestNewApp_NilConfig(t *testing.T) {
	_, err := NewApp(nil)
	if err == nil {
		t.Fatal("expected error for nil config")
	}
	if !strings.Contains(err.Error(), "config is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewApp_UnknownProfile(t *testing.T) {
	cfg := &Config{Display: "nonexistent", Backend: "preview"}
	_, err := NewApp(cfg)
	if err == nil {
		t.Fatal("expected error for unknown profile")
	}
	if !strings.Contains(err.Error(), "unknown display profile") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRun_RendersAndStops(t *testing.T) {
	cfg, err := LoadConfig(strings.NewReader(`
display: waveshare_7in5_v2
backend: preview
`))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	mock := &MockHardware{}
	app, err := NewApp(cfg, WithHardware(mock), WithInterval(10*time.Millisecond))
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if err := app.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Verify lifecycle: should see reset (from Init), display commands, and close
	calls := mock.Calls
	if len(calls) == 0 {
		t.Fatal("expected hardware calls, got none")
	}

	// First call should be reset (from EPD.Init)
	if calls[0].Type != "reset" {
		t.Errorf("first call should be reset, got %s", calls[0].Type)
	}

	// Last call should be close (from EPD.Close -> hw.Close)
	last := calls[len(calls)-1]
	if last.Type != "close" {
		t.Errorf("last call should be close, got %s", last.Type)
	}

	// Should have at least one refresh command (0x12) from display
	cmds := mock.Commands()
	hasRefresh := slices.Contains(cmds, 0x12)
	if !hasRefresh {
		t.Error("expected at least one refresh command (0x12)")
	}
}

// brokenWidget is a Widget that always returns an error on Render.
type brokenWidget struct {
	bounds image.Rectangle
}

func (e *brokenWidget) Bounds() image.Rectangle      { return e.bounds }
func (e *brokenWidget) Render(*image.Paletted) error { return fmt.Errorf("widget broke") }

func TestRun_WidgetRenderError(t *testing.T) {
	cfg, _ := LoadConfig(strings.NewReader(`
display: waveshare_7in5_v2
backend: preview
`))

	mock := &MockHardware{}
	app, err := NewApp(cfg, WithHardware(mock), WithInterval(time.Millisecond))
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}

	app.comp.AddWidget(&brokenWidget{bounds: image.Rect(0, 0, 10, 10)})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err = app.Run(ctx)
	if err == nil {
		t.Fatal("expected error from broken widget")
	}
	if !strings.Contains(err.Error(), "widget broke") {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should still close hardware on error
	last := mock.Calls[len(mock.Calls)-1]
	if last.Type != "close" {
		t.Errorf("expected close after error, got %s", last.Type)
	}
}

func TestCreateBackend_Preview(t *testing.T) {
	cfg := &Config{Backend: "preview"}
	profile := &Waveshare7in5V2
	hw, err := createBackend(cfg, profile)
	if err != nil {
		t.Fatalf("createBackend: %v", err)
	}
	if _, ok := hw.(*WebPreview); !ok {
		t.Errorf("expected *WebPreview, got %T", hw)
	}
}

func TestCreateBackend_Image(t *testing.T) {
	cfg := &Config{Backend: "image", Image: ImageConfig{OutputDir: "/tmp"}}
	profile := &Waveshare7in5V2
	hw, err := createBackend(cfg, profile)
	if err != nil {
		t.Fatalf("createBackend: %v", err)
	}
	if _, ok := hw.(*ImageBackend); !ok {
		t.Errorf("expected *ImageBackend, got %T", hw)
	}
}

func TestCreateBackend_Unsupported(t *testing.T) {
	cfg := &Config{Backend: "unknown"}
	profile := &Waveshare7in5V2
	_, err := createBackend(cfg, profile)
	if err == nil {
		t.Fatal("expected error for unsupported backend")
	}
	if !strings.Contains(err.Error(), "unsupported backend") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewApp_DefaultBackendPreview(t *testing.T) {
	cfg, _ := LoadConfig(strings.NewReader(`
display: waveshare_7in5_v2
backend: preview
`))
	app, err := NewApp(cfg)
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	if app == nil {
		t.Fatal("expected non-nil App")
	}
}

func TestRun_MultipleRenderCycles(t *testing.T) {
	cfg, _ := LoadConfig(strings.NewReader(`
display: waveshare_7in5_v2
backend: preview
`))

	mock := &MockHardware{}
	app, err := NewApp(cfg, WithHardware(mock), WithInterval(time.Millisecond))
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	if err := app.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Count refresh commands — should have more than one render cycle
	cmds := mock.Commands()
	refreshCount := 0
	for _, c := range cmds {
		if c == 0x12 {
			refreshCount++
		}
	}
	if refreshCount < 2 {
		t.Errorf("expected multiple render cycles, got %d refresh commands", refreshCount)
	}
}

func TestNewApp_UnsupportedBackendNoOverride(t *testing.T) {
	// "spi" passes config validation but createBackend doesn't support it yet
	cfg, _ := LoadConfig(strings.NewReader(`
display: waveshare_7in5_v2
backend: spi
`))
	_, err := NewApp(cfg)
	if err == nil {
		t.Fatal("expected error for unsupported backend")
	}
	if !strings.Contains(err.Error(), "unsupported backend") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// resetFailHardware is a mock that fails on Reset (used to test Init error path).
type resetFailHardware struct{ MockHardware }

func (e *resetFailHardware) Reset() error { return fmt.Errorf("hardware reset failed") }

func TestRun_InitError(t *testing.T) {
	cfg, _ := LoadConfig(strings.NewReader(`
display: waveshare_7in5_v2
backend: preview
`))

	mock := &resetFailHardware{}
	app, err := NewApp(cfg, WithHardware(mock), WithInterval(time.Millisecond))
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}

	err = app.Run(context.Background())
	if err == nil {
		t.Fatal("expected init error")
	}
	if !strings.Contains(err.Error(), "init display") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// displayErrorHardware fails on SendCommand for the refresh command (0x12),
// simulating a display error during the Display call.
type displayErrorHardware struct {
	MockHardware
	failOnRefresh bool
}

func (d *displayErrorHardware) SendCommand(cmd byte) error {
	if d.failOnRefresh && cmd == 0x12 {
		return fmt.Errorf("display send failed")
	}
	return d.MockHardware.SendCommand(cmd)
}

func TestRun_DisplayError(t *testing.T) {
	cfg, _ := LoadConfig(strings.NewReader(`
display: waveshare_7in5_v2
backend: preview
`))

	mock := &displayErrorHardware{failOnRefresh: true}
	app, err := NewApp(cfg, WithHardware(mock), WithInterval(time.Millisecond))
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}

	err = app.Run(context.Background())
	if err == nil {
		t.Fatal("expected display error")
	}
	if !strings.Contains(err.Error(), "display") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewApp_DefaultBackendImage(t *testing.T) {
	cfg, _ := LoadConfig(strings.NewReader(`
display: waveshare_7in5_v2
backend: image
image:
  output_dir: /tmp/inkwell-test
`))
	app, err := NewApp(cfg)
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	if app == nil {
		t.Fatal("expected non-nil App")
	}
}
