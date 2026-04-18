package inkwell

import (
	"context"
	"fmt"
	"image"
	"net"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

// Compile-time assertion: WebPreview satisfies HTTPServer.
var _ HTTPServer = (*WebPreview)(nil)

func TestRun_StartsHTTPServer(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Preview.Port = 0
	wp := NewWebPreview(&Waveshare7in5V2)

	app, err := NewApp(cfg, WithHardware(wp), WithInterval(10*time.Millisecond))
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- app.Run(ctx) }()

	<-app.Ready()
	addr := app.Addr()
	if addr == nil {
		t.Fatal("expected non-nil addr")
	}

	resp, err := http.Get(fmt.Sprintf("http://%s/", addr))
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func TestReady_ClosedWithoutHTTPServer(t *testing.T) {
	cfg := DefaultConfig()
	mock := &MockHardware{}
	app, err := NewApp(cfg, WithHardware(mock), WithInterval(10*time.Millisecond))
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	go app.Run(ctx)

	select {
	case <-app.Ready():
		// good
	case <-time.After(time.Second):
		t.Fatal("Ready() not closed in time")
	}

	if app.Addr() != nil {
		t.Fatal("expected nil Addr for non-HTTPServer hardware")
	}
}

func TestRun_ServerShutdownOnCancel(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Preview.Port = 0
	wp := NewWebPreview(&Waveshare7in5V2)

	app, err := NewApp(cfg, WithHardware(wp), WithInterval(10*time.Millisecond))
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- app.Run(ctx) }()

	<-app.Ready()
	addr := app.Addr().String()

	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Server should be shut down — connection refused.
	_, err = http.Get("http://" + addr + "/")
	if err == nil {
		t.Fatal("expected connection refused after shutdown")
	}
}

func TestRun_ListenError(t *testing.T) {
	// Occupy a port so the app's Listen fails.
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("pre-listen: %v", err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	cfg := DefaultConfig()
	cfg.Preview.Port = port
	wp := NewWebPreview(&Waveshare7in5V2)

	app, err := NewApp(cfg, WithHardware(wp), WithInterval(10*time.Millisecond))
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}

	err = app.Run(context.Background())
	if err == nil {
		t.Fatal("expected listen error")
	}
	if !strings.Contains(err.Error(), "listen") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRun_ServerCrashAbortsLoop(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Preview.Port = 0
	wp := NewWebPreview(&Waveshare7in5V2)

	app, err := NewApp(cfg, WithHardware(wp), WithInterval(time.Hour))
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}

	errCh := make(chan error, 1)
	go func() { errCh <- app.Run(context.Background()) }()

	<-app.Ready()
	// Force Serve to return an error by closing the listener externally.
	app.listener.Close()

	err = <-errCh
	if err == nil {
		t.Fatal("expected server error")
	}
	if !strings.Contains(err.Error(), "preview server") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewApp_DefaultListenAddr(t *testing.T) {
	cfg := DefaultConfig()
	mock := &MockHardware{}
	app, err := NewApp(cfg, WithHardware(mock), WithInterval(time.Millisecond))
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	if app.listenAddr != ":8080" {
		t.Errorf("listenAddr = %q, want %q", app.listenAddr, ":8080")
	}
}

func TestNewApp_CustomPort(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Preview.Port = 3000
	mock := &MockHardware{}
	app, err := NewApp(cfg, WithHardware(mock), WithInterval(time.Millisecond))
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	if app.listenAddr != ":3000" {
		t.Errorf("listenAddr = %q, want %q", app.listenAddr, ":3000")
	}
}

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

func TestNewApp_ZeroInterval(t *testing.T) {
	cfg := &Config{Display: "waveshare_7in5_v2", Backend: "preview"}
	_, err := NewApp(cfg, WithInterval(0))
	if err == nil {
		t.Fatal("expected error for zero interval")
	}
	if !strings.Contains(err.Error(), "interval must be positive") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewApp_NegativeInterval(t *testing.T) {
	cfg := &Config{Display: "waveshare_7in5_v2", Backend: "preview"}
	_, err := NewApp(cfg, WithInterval(-time.Second))
	if err == nil {
		t.Fatal("expected error for negative interval")
	}
	if !strings.Contains(err.Error(), "interval must be positive") {
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

	// Verify full framebuffer is transmitted in a single SendData call
	expectedSize := Waveshare7in5V2.BufferSize()
	hasFullFrame := false
	for _, c := range calls {
		if c.Type == "data" && len(c.Data) == expectedSize {
			hasFullFrame = true
			break
		}
	}
	if !hasFullFrame {
		t.Errorf("expected at least one SendData call with full framebuffer size %d", expectedSize)
	}
}

// brokenWidget is a Widget that always returns an error on Render.
type brokenWidget struct {
	bounds image.Rectangle
}

func (e *brokenWidget) Bounds() image.Rectangle      { return e.bounds }
func (e *brokenWidget) Render(*image.Paletted) error { return fmt.Errorf("widget broke") }

func TestRun_WidgetRenderError(t *testing.T) {
	cfg, err := LoadConfig(strings.NewReader(`
display: waveshare_7in5_v2
backend: preview
dashboard:
  screens:
    - name: main
      widgets:
        - type: broken
          bounds: [0, 0, 10, 10]
`))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	reg := widget.NewRegistry()
	reg.Register("broken", func(bounds image.Rectangle, _ map[string]any, _ widget.Deps) (widget.Widget, error) {
		return &brokenWidget{bounds: bounds}, nil
	})

	mock := &MockHardware{}
	app, err := NewApp(cfg, WithHardware(mock), WithInterval(time.Millisecond), WithRegistry(reg))
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}

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

func TestBuildDashboard_EmptyConfig(t *testing.T) {
	cfg := DefaultConfig()
	profile := &Waveshare7in5V2
	registry := widget.NewRegistry()
	deps := widget.Deps{Now: time.Now}

	d, err := buildDashboard(cfg, profile, registry, deps)
	if err != nil {
		t.Fatalf("buildDashboard: %v", err)
	}
	screen := d.CurrentScreen()
	if screen == nil {
		t.Fatal("expected default screen")
	}
	if screen.Name != "default" {
		t.Errorf("Name = %q, want default", screen.Name)
	}
}

func TestBuildDashboard_WithWidgets(t *testing.T) {
	cfg, err := LoadConfig(strings.NewReader(`
display: waveshare_7in5_v2
backend: preview
dashboard:
  screens:
    - name: main
      widgets:
        - type: clock
          bounds: [0, 0, 200, 50]
          config:
            format: "15:04"
`))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	fixedTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	deps := widget.Deps{Now: func() time.Time { return fixedTime }}
	profile := &Waveshare7in5V2

	reg := widget.NewRegistry()
	reg.Register("clock", func(bounds image.Rectangle, config map[string]any, deps widget.Deps) (widget.Widget, error) {
		return &stubWidget{bounds: bounds}, nil
	})

	d, err := buildDashboard(cfg, profile, reg, deps)
	if err != nil {
		t.Fatalf("buildDashboard: %v", err)
	}
	screen := d.CurrentScreen()
	if len(screen.Widgets()) != 1 {
		t.Fatalf("len(Widgets) = %d, want 1", len(screen.Widgets()))
	}
}

func TestBuildDashboard_EmptyBounds(t *testing.T) {
	cfg, err := LoadConfig(strings.NewReader(`
display: waveshare_7in5_v2
backend: preview
dashboard:
  screens:
    - name: main
      widgets:
        - type: stub
          bounds: [0, 0, 0, 0]
`))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	reg := widget.NewRegistry()
	reg.Register("stub", func(bounds image.Rectangle, _ map[string]any, _ widget.Deps) (widget.Widget, error) {
		return &stubWidget{bounds: bounds}, nil
	})

	_, err = buildDashboard(cfg, &Waveshare7in5V2, reg, widget.Deps{Now: time.Now})
	if err == nil {
		t.Fatal("expected error for empty bounds")
	}
	if !strings.Contains(err.Error(), "empty bounds") {
		t.Errorf("error = %q, want mention of empty bounds", err.Error())
	}
}

func TestBuildDashboard_BoundsExceedDisplay(t *testing.T) {
	cfg, err := LoadConfig(strings.NewReader(`
display: waveshare_7in5_v2
backend: preview
dashboard:
  screens:
    - name: main
      widgets:
        - type: stub
          bounds: [0, 0, 900, 50]
`))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	reg := widget.NewRegistry()
	reg.Register("stub", func(bounds image.Rectangle, _ map[string]any, _ widget.Deps) (widget.Widget, error) {
		return &stubWidget{bounds: bounds}, nil
	})

	_, err = buildDashboard(cfg, &Waveshare7in5V2, reg, widget.Deps{Now: time.Now})
	if err == nil {
		t.Fatal("expected error for out-of-bounds widget")
	}
	if !strings.Contains(err.Error(), "exceed display") {
		t.Errorf("error = %q, want mention of exceed display", err.Error())
	}
}

func TestBuildDashboard_UnknownWidgetType(t *testing.T) {
	cfg, err := LoadConfig(strings.NewReader(`
display: waveshare_7in5_v2
backend: preview
dashboard:
  screens:
    - name: main
      widgets:
        - type: nonexistent
          bounds: [0, 0, 100, 50]
`))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	reg := widget.NewRegistry()
	_, err = buildDashboard(cfg, &Waveshare7in5V2, reg, widget.Deps{Now: time.Now})
	if err == nil {
		t.Fatal("expected error for unknown widget type")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error = %q, want mention of nonexistent", err.Error())
	}
}

func TestNewApp_DashboardBuildError(t *testing.T) {
	cfg, err := LoadConfig(strings.NewReader(`
display: waveshare_7in5_v2
backend: preview
dashboard:
  screens:
    - name: main
      widgets:
        - type: nonexistent
          bounds: [0, 0, 100, 50]
`))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	mock := &MockHardware{}
	_, err = NewApp(cfg, WithHardware(mock), WithInterval(time.Millisecond))
	if err == nil {
		t.Fatal("expected error for unknown widget type")
	}
	if !strings.Contains(err.Error(), "build dashboard") {
		t.Errorf("error = %q, want mention of build dashboard", err.Error())
	}
}

func TestNewApp_WithDeps(t *testing.T) {
	fixedTime := time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)
	cfg, err := LoadConfig(strings.NewReader(`
display: waveshare_7in5_v2
backend: preview
`))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	mock := &MockHardware{}
	deps := widget.Deps{Now: func() time.Time { return fixedTime }}
	_, err = NewApp(cfg, WithHardware(mock), WithInterval(time.Millisecond), WithDeps(deps))
	if err != nil {
		t.Fatalf("NewApp with WithDeps: %v", err)
	}
}

func TestNewApp_WithDashboardConfig(t *testing.T) {
	cfg, err := LoadConfig(strings.NewReader(`
display: waveshare_7in5_v2
backend: preview
dashboard:
  screens:
    - name: home
      widgets:
        - type: clock
          bounds: [0, 0, 200, 50]
`))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	mock := &MockHardware{}
	app, err := NewApp(cfg,
		WithHardware(mock),
		WithInterval(time.Millisecond),
	)
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	if app.dashboard == nil {
		t.Fatal("dashboard is nil")
	}
	screen := app.dashboard.CurrentScreen()
	if screen.Name != "home" {
		t.Errorf("screen name = %q, want home", screen.Name)
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
	cfg, err := LoadConfig(strings.NewReader(`
display: waveshare_7in5_v2
backend: preview
`))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	app, err := NewApp(cfg)
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	if app == nil {
		t.Fatal("expected non-nil App")
	}
}

func TestRun_MultipleRenderCycles(t *testing.T) {
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

// resetFailHardware is a mock that fails on Reset (used to test Init error path).
type resetFailHardware struct{ MockHardware }

func (e *resetFailHardware) Reset() error { return fmt.Errorf("hardware reset failed") }

func TestRun_InitError(t *testing.T) {
	cfg, err := LoadConfig(strings.NewReader(`
display: waveshare_7in5_v2
backend: preview
`))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

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
	cfg, err := LoadConfig(strings.NewReader(`
display: waveshare_7in5_v2
backend: preview
`))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

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

func TestRun_PackImageError(t *testing.T) {
	mock := &MockHardware{}
	// BW profile for compositor (Render succeeds), Color7 profile for PackImage (fails).
	bwProfile := DisplayProfile{
		Width:    16,
		Height:   16,
		Color:    BW,
		InitFull: []Command{{0x00, nil}},
	}
	color7Profile := DisplayProfile{
		Width:    16,
		Height:   16,
		Color:    Color7,
		InitFull: []Command{{0x00, nil}},
	}
	app := &App{
		hw:        mock,
		epd:       NewEPD(mock, &bwProfile),
		comp:      NewCompositor(&bwProfile),
		profile:   &color7Profile,
		dashboard: NewDashboard([]*Screen{NewScreen("default", nil)}, 0, time.Now),
		interval:  time.Millisecond,
	}

	err := app.Run(context.Background())
	if err == nil {
		t.Fatal("expected pack image error")
	}
	if !strings.Contains(err.Error(), "pack image") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRun_ShutdownTimeoutFallsBackToClose(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Preview.Port = 0
	wp := NewWebPreview(&Waveshare7in5V2)

	app, err := NewApp(cfg, WithHardware(wp), WithInterval(10*time.Millisecond))
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	// Force shutdown to time out immediately so Close() fallback is exercised.
	app.shutdownTimeout = time.Nanosecond

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- app.Run(ctx) }()

	<-app.Ready()

	// Open a long-lived SSE connection to keep the server busy during shutdown.
	addr := app.Addr().String()
	sseCtx := t.Context()
	req, _ := http.NewRequestWithContext(sseCtx, "GET", "http://"+addr+"/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /events: %v", err)
	}
	defer resp.Body.Close()

	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func TestNewApp_DefaultBackendImage(t *testing.T) {
	cfg, err := LoadConfig(strings.NewReader(`
display: waveshare_7in5_v2
backend: image
image:
  output_dir: /tmp/inkwell-test
`))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	app, err := NewApp(cfg)
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	if app == nil {
		t.Fatal("expected non-nil App")
	}
}
