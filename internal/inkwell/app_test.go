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

// TestRun_Init4GrayWhenColorGray4 confirms App.Run picks the Init4Gray
// init sequence (not InitFull) when the resolved profile is Gray4. The
// signature command is 0xE5 with data 0x5F — Waveshare's "force
// temperature for 4-gray refresh waveform." Without this, the panel
// would receive a BW init followed by 4-gray plane data and render
// garbage.
func TestRun_Init4GrayWhenColorGray4(t *testing.T) {
	cfg, err := LoadConfig(strings.NewReader(`
display: waveshare_7in5_v2
backend: preview
color_mode: gray4
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

	// Find the 0xE5 (force temperature) command and confirm its data byte.
	// Init4Gray uses 0x5F (4-gray); InitFull doesn't issue 0xE5 at all.
	var foundTempCmd bool
	for i, c := range mock.Calls {
		if c.Type == "command" && c.Data[0] == 0xE5 {
			if i+1 >= len(mock.Calls) || mock.Calls[i+1].Type != "data" {
				t.Fatalf("0xE5 command at call %d missing data payload", i)
			}
			data := mock.Calls[i+1].Data
			if len(data) != 1 || data[0] != 0x5F {
				t.Errorf("0xE5 data = % X, want 0x5F (4-gray temperature)", data)
			}
			foundTempCmd = true
			break
		}
	}
	if !foundTempCmd {
		t.Fatal("init sequence missing 0xE5 — Gray4 init was not selected")
	}

	// And the first init command after reset should match the Init4Gray
	// sequence, not InitFull's. Init4Gray starts with 0x00 (panel
	// setting); InitFull starts with 0x06 (booster soft start).
	var firstCmd byte
	for _, c := range mock.Calls {
		if c.Type == "command" {
			firstCmd = c.Data[0]
			break
		}
	}
	if firstCmd != 0x00 {
		t.Errorf("first init command = 0x%X, want 0x00 (Init4Gray panel setting)", firstCmd)
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

func TestBuildDashboard_RotateIntervalWithoutScreens(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Dashboard.RotateInterval = Duration(5 * time.Minute)
	// No screens configured

	_, err := buildDashboard(cfg, &Waveshare7in5V2, widget.NewRegistry(), widget.Deps{Now: time.Now})
	if err == nil {
		t.Fatal("expected error for rotate_interval without screens")
	}
	if !strings.Contains(err.Error(), "no screens are configured") {
		t.Errorf("error = %q, want mention of no screens", err.Error())
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

	cfg.Dashboard = DashboardConfig{Screens: []ScreenConfig{{Name: "main", Widgets: []WidgetConfig{{Type: "probe", Bounds: [4]int{0, 0, 10, 10}}}}}}
	reg := widget.NewRegistry()
	var got time.Time
	reg.Register("probe", func(b image.Rectangle, _ map[string]any, d widget.Deps) (widget.Widget, error) {
		got = d.Now()
		return &stubWidget{bounds: b}, nil
	})

	mock := &MockHardware{}
	deps := widget.Deps{Now: func() time.Time { return fixedTime }}
	_, err = NewApp(cfg, WithHardware(mock), WithInterval(time.Millisecond), WithRegistry(reg), WithDeps(deps))
	if err != nil {
		t.Fatalf("NewApp with WithDeps: %v", err)
	}
	if !got.Equal(fixedTime) {
		t.Errorf("deps.Now propagated time = %v, want %v", got, fixedTime)
	}
}

// A caller-injected DataSources["http_client"] that doesn't satisfy
// weather.HTTPClient must still produce a working App: NewApp falls
// back to http.DefaultClient for the default weather_source rather
// than panicking at first request.
func TestNewApp_HTTPClientDoesNotSatisfyWeatherClient(t *testing.T) {
	cfg, err := LoadConfig(strings.NewReader(`
display: waveshare_7in5_v2
backend: preview
`))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	mock := &MockHardware{}
	deps := widget.Deps{
		DataSources: map[string]any{
			// A string is the simplest "doesn't satisfy weather.HTTPClient" value.
			"http_client": "not-a-client",
		},
	}
	app, err := NewApp(cfg, WithHardware(mock), WithInterval(time.Millisecond), WithDeps(deps))
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	if app == nil {
		t.Fatal("NewApp returned nil")
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

func TestNewApp_ColorModeBW_LeavesProfileBW(t *testing.T) {
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
	if app.profile.Color != BW {
		t.Errorf("profile.Color = %v, want BW", app.profile.Color)
	}
	// The global Profiles entry must not be mutated.
	if Profiles["waveshare_7in5_v2"].Color != BW {
		t.Errorf("global profile mutated: Color = %v", Profiles["waveshare_7in5_v2"].Color)
	}
}

func TestNewApp_ColorModeGray4_OverridesProfile(t *testing.T) {
	cfg, err := LoadConfig(strings.NewReader(`
display: waveshare_7in5_v2
backend: preview
color_mode: gray4
`))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	mock := &MockHardware{}
	app, err := NewApp(cfg, WithHardware(mock), WithInterval(time.Millisecond))
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	if app.profile.Color != Gray4 {
		t.Errorf("profile.Color = %v, want Gray4", app.profile.Color)
	}
	// EPD and Compositor must observe the overridden profile.
	if app.epd.profile.Color != Gray4 {
		t.Errorf("epd profile.Color = %v, want Gray4", app.epd.profile.Color)
	}
	if app.comp.profile.Color != Gray4 {
		t.Errorf("compositor profile.Color = %v, want Gray4", app.comp.profile.Color)
	}
	// The global Profiles entry must remain BW.
	if Profiles["waveshare_7in5_v2"].Color != BW {
		t.Errorf("global profile mutated: Color = %v", Profiles["waveshare_7in5_v2"].Color)
	}
}

func TestNewApp_ColorModeGray4_UnsupportedByProfile(t *testing.T) {
	bwOnly := DisplayProfile{
		Name:         "bw_only_test",
		Width:        16,
		Height:       16,
		Color:        BW,
		Capabilities: Capabilities{Grayscale: false},
		InitFull:     []Command{{0x00, nil}},
	}
	Profiles["bw_only_test"] = &bwOnly
	t.Cleanup(func() { delete(Profiles, "bw_only_test") })

	cfg := &Config{Display: "bw_only_test", Backend: "preview", ColorMode: "gray4"}
	mock := &MockHardware{}
	_, err := NewApp(cfg, WithHardware(mock), WithInterval(time.Millisecond))
	if err == nil {
		t.Fatal("expected error for gray4 on non-grayscale profile")
	}
	if !strings.Contains(err.Error(), "grayscale") {
		t.Errorf("error = %q, want mention of grayscale", err.Error())
	}
}

func TestApplyColorMode(t *testing.T) {
	base := &DisplayProfile{
		Name:         "test_base",
		Color:        BW,
		Capabilities: Capabilities{Grayscale: true},
	}

	cases := []struct {
		label     string
		mode      string
		wantColor ColorDepth
		wantSame  bool // expect returned pointer == base (no copy)
		wantErr   string
	}{
		{label: "empty defaults to bw", mode: "", wantColor: BW, wantSame: true},
		{label: "explicit bw", mode: "bw", wantColor: BW, wantSame: true},
		{label: "gray4 copies and overrides", mode: "gray4", wantColor: Gray4},
		{label: "rejects unknown", mode: "color7", wantErr: "invalid color_mode"},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			got, err := applyColorMode(base, tc.mode)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error mentioning %q, got nil", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("error = %q, want mention of %q", err.Error(), tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("applyColorMode: %v", err)
			}
			if got.Color != tc.wantColor {
				t.Errorf("Color = %v, want %v", got.Color, tc.wantColor)
			}
			if tc.wantSame && got != base {
				t.Errorf("expected same pointer for mode %q, got a copy", tc.mode)
			}
			if !tc.wantSame && got == base {
				t.Errorf("expected a copy for mode %q, got same pointer", tc.mode)
			}
		})
	}
}

// TestRun_GracefulShutdownClearsBeforeSleep covers the contract:
// on signal-driven shutdown (ctx.Done), the EPD is cleared to white
// and the clear's refresh (with BUSY wait) completes BEFORE the
// SleepSequence is sent — otherwise the panel would retain the
// pre-clear frame after power-down.
//
// Long interval + pre-cancelled context guarantees exactly one render
// before the cleanup path runs, so refresh count = 1 (render) + 1 (Clear) = 2.
func TestRun_GracefulShutdownClearsBeforeSleep(t *testing.T) {
	cfg := DefaultConfig() // ClearOnShutdown defaults to true
	mock := &MockHardware{}
	app, err := NewApp(cfg, WithHardware(mock), WithInterval(time.Hour))
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := app.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}

	refreshCount := 0
	for _, b := range mock.Commands() {
		if b == 0x12 {
			refreshCount++
		}
	}
	if refreshCount != 2 {
		t.Errorf("refresh count = %d, want 2 (1 render + 1 clear)", refreshCount)
	}

	// The last 0x12 (Clear's refresh) must be followed by busy, then the
	// SleepSequence commands, then close.
	lastRefresh := -1
	for i, c := range mock.Calls {
		if c.Type == "command" && c.Data[0] == 0x12 {
			lastRefresh = i
		}
	}
	if lastRefresh < 0 {
		t.Fatal("no refresh command found")
	}

	after := mock.Calls[lastRefresh+1:]
	want := []struct {
		typ  string
		cmd  byte
		data []byte
	}{
		{typ: "busy"},                              // Display's ReadBusy after refresh
		{typ: "command", cmd: 0x50},                // sleep VCOM setting
		{typ: "data", data: []byte{0xF7}},          // VCOM data
		{typ: "command", cmd: 0x02},                // power off
		{typ: "busy"},                              // execSequence busy after nil-data
		{typ: "command", cmd: 0x07},                // deep sleep
		{typ: "data", data: []byte{0xA5}},          // deep sleep data
		{typ: "close"},                             // hw.Close
	}
	if len(after) < len(want) {
		t.Fatalf("calls after last refresh = %d, want >= %d", len(after), len(want))
	}
	for i, w := range want {
		got := after[i]
		if got.Type != w.typ {
			t.Errorf("after[%d].Type = %q, want %q", i, got.Type, w.typ)
			continue
		}
		switch w.typ {
		case "command":
			if got.Data[0] != w.cmd {
				t.Errorf("after[%d] command = 0x%02X, want 0x%02X", i, got.Data[0], w.cmd)
			}
		case "data":
			if len(got.Data) != len(w.data) || got.Data[0] != w.data[0] {
				t.Errorf("after[%d] data = % X, want % X", i, got.Data, w.data)
			}
		}
	}
}

// TestRun_ClearOnShutdownDisabled confirms the opt-out: when
// clear_on_shutdown is false, the cleanup path skips Clear entirely
// and only the render's single refresh shows up.
func TestRun_ClearOnShutdownDisabled(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ClearOnShutdown = false
	mock := &MockHardware{}
	app, err := NewApp(cfg, WithHardware(mock), WithInterval(time.Hour))
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := app.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}

	refreshCount := 0
	for _, b := range mock.Commands() {
		if b == 0x12 {
			refreshCount++
		}
	}
	if refreshCount != 1 {
		t.Errorf("refresh count = %d, want 1 (clear disabled)", refreshCount)
	}
}

// TestRun_CrashPathDoesNotClear confirms Clear runs ONLY on the
// ctx.Done branch — never on render/display/server errors. If Clear
// were called on the widget-render error path, we'd see a 0x12
// refresh from Clear's Display call.
func TestRun_CrashPathDoesNotClear(t *testing.T) {
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

	if err := app.Run(context.Background()); err == nil {
		t.Fatal("expected render error")
	}

	for _, b := range mock.Commands() {
		if b == 0x12 {
			t.Fatal("Clear should not run on crash paths; saw refresh 0x12")
		}
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
