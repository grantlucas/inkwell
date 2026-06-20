package inkwell

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets"
)

// HTTPServer is implemented by Hardware backends that also serve HTTP.
// App.Run starts an http.Server when the backend satisfies this interface.
type HTTPServer interface {
	Handler() http.Handler
}

// App wires together config, backend, EPD, and compositor into a running
// application. Use NewApp to construct, then Run to start the render loop.
type App struct {
	hw              Hardware
	epd             *EPD
	comp            *Compositor
	profile         *DisplayProfile
	dashboard       *Dashboard
	planner         *refreshPlanner
	now             func() time.Time
	interval        time.Duration
	listenAddr      string
	listener        net.Listener
	ready           chan struct{}
	shutdownTimeout time.Duration
	clearOnShutdown bool
}

// AppOption configures optional App parameters.
type AppOption func(*appOptions)

type appOptions struct {
	hw       Hardware
	interval time.Duration
	registry *widget.Registry
	deps     widget.Deps
}

// WithHardware injects a Hardware backend, overriding config-driven selection.
func WithHardware(hw Hardware) AppOption {
	return func(o *appOptions) { o.hw = hw }
}

// WithInterval sets the render loop sleep interval.
func WithInterval(d time.Duration) AppOption {
	return func(o *appOptions) { o.interval = d }
}

// WithRegistry injects a widget registry, overriding the default.
func WithRegistry(r *widget.Registry) AppOption {
	return func(o *appOptions) { o.registry = r }
}

// WithDeps injects widget dependencies, overriding defaults.
func WithDeps(d widget.Deps) AppOption {
	return func(o *appOptions) { o.deps = d }
}

// NewApp creates an App from config. It resolves the display profile, creates
// the hardware backend (unless overridden via WithHardware), and wires up the
// EPD and compositor.
func NewApp(cfg *Config, opts ...AppOption) (*App, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}

	o := &appOptions{
		interval: 60 * time.Second,
	}
	for _, fn := range opts {
		fn(o)
	}
	if o.interval <= 0 {
		return nil, fmt.Errorf("interval must be positive")
	}

	baseProfile, ok := Profiles[cfg.Display]
	if !ok {
		return nil, fmt.Errorf("unknown display profile: %q", cfg.Display)
	}

	profile, err := applyColorMode(baseProfile, cfg.ColorMode)
	if err != nil {
		return nil, err
	}

	hw := o.hw
	if hw == nil {
		var err error
		hw, err = createBackend(cfg, profile)
		if err != nil {
			return nil, err
		}
	}

	registry := o.registry
	if registry == nil {
		registry = widgets.NewDefaultRegistry()
	}
	deps := o.deps
	if deps.Now == nil {
		deps.Now = time.Now
	}
	if deps.DataSources == nil {
		deps.DataSources = make(map[string]any)
	}
	if _, ok := deps.DataSources["http_client"]; !ok {
		deps.DataSources["http_client"] = http.DefaultClient
	}
	if _, ok := deps.DataSources["weather_source"]; !ok {
		// Use the injected http_client so callers can override the
		// transport (timeouts, instrumentation, test stubs) and have
		// the default weather source actually honor it.
		httpClient, ok := deps.DataSources["http_client"].(weather.HTTPClient)
		if !ok {
			httpClient = http.DefaultClient
		}
		ensemble := weather.NewEnsembleSource(
			weather.NewOpenMeteoSource(weather.ModelGFS, httpClient),
			weather.NewOpenMeteoSource(weather.ModelECMWF, httpClient),
			weather.NewOpenMeteoSource(weather.ModelGEM, httpClient),
		)
		deps.DataSources["weather_source"] = weather.NewCachedSource(
			ensemble, 3*time.Hour, deps.Now,
		)
	}

	dashboard, err := buildDashboard(cfg, profile, registry, deps)
	if err != nil {
		return nil, fmt.Errorf("build dashboard: %w", err)
	}

	epd := NewEPD(hw, profile)
	comp := NewCompositor(profile)

	return &App{
		hw:              hw,
		epd:             epd,
		comp:            comp,
		profile:         profile,
		dashboard:       dashboard,
		planner:         newRefreshPlanner(profile.Color, defaultFullEvery, defaultFastEvery),
		now:             deps.Now,
		interval:        o.interval,
		listenAddr:      fmt.Sprintf(":%d", cfg.Preview.Port),
		ready:           make(chan struct{}),
		shutdownTimeout: 5 * time.Second,
		clearOnShutdown: cfg.ClearOnShutdown,
	}, nil
}

// Ready returns a channel that is closed once the app is fully started
// (including the HTTP server listener, if any).
func (a *App) Ready() <-chan struct{} { return a.ready }

// Addr returns the listener address when the hardware backend serves HTTP,
// or nil otherwise. Only valid after Ready is closed.
func (a *App) Addr() net.Addr {
	if a.listener != nil {
		return a.listener.Addr()
	}
	return nil
}

// Run initializes the display, enters the render loop, and shuts down when
// ctx is cancelled. The loop: compose → pack → display → sleep. When the
// hardware backend implements HTTPServer, an HTTP server is started
// concurrently.
func (a *App) Run(ctx context.Context) error {
	var closeReady sync.Once
	signalReady := func() {
		closeReady.Do(func() {
			if a.ready != nil {
				close(a.ready)
			}
		})
	}
	defer signalReady()

	mode := initModeFor(a.profile.Color)
	if err := a.epd.Init(mode); err != nil {
		a.epd.Close()
		return fmt.Errorf("init display %q: %w", a.profile.Name, err)
	}

	// Start HTTP server if the backend supports it.
	var serverErr <-chan error
	if hs, ok := a.hw.(HTTPServer); ok {
		ln, err := net.Listen("tcp", a.listenAddr)
		if err != nil {
			a.epd.Close()
			return fmt.Errorf("listen: %w", err)
		}
		a.listener = ln

		srv := &http.Server{Handler: hs.Handler()}
		done := make(chan struct{})
		ch := make(chan error, 1)
		go func() {
			defer close(done)
			if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
				ch <- err
			}
		}()
		serverErr = ch
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), a.shutdownTimeout)
			defer cancel()
			if err := srv.Shutdown(shutdownCtx); err != nil {
				_ = srv.Close()
			}
			<-done
		}()
	}
	signalReady()

	ticker := time.NewTicker(a.interval)
	defer ticker.Stop()

	// appliedMode tracks the init sequence (waveform LUT) currently loaded so
	// we only re-init when the refresh mode actually changes — re-initing
	// every cycle would add a needless reset. lastBuffer is the frame last
	// pushed to the panel, used both to detect unchanged content and to feed
	// the controller's old plane on a partial refresh.
	appliedMode := mode
	var lastBuffer []byte
	fullWindow := Region{X: 0, Y: 0, W: a.profile.Width, H: a.profile.Height}

	for {
		var ws []widget.Widget
		due := false
		if screen := a.dashboard.CurrentScreen(); screen != nil {
			ws = screen.Widgets()
			due = screen.AnyDue(a.now())
		}

		frame, err := a.comp.Render(ws)
		if err != nil {
			a.epd.Close()
			return fmt.Errorf("render: %w", err)
		}

		if sink, ok := a.hw.(FrameSink); ok {
			sink.SetSourceFrame(frame)
		}

		buf, err := PackImage(a.profile, frame)
		if err != nil {
			a.epd.Close()
			return fmt.Errorf("pack image: %w", err)
		}

		pushed, err := a.refresh(buf, lastBuffer, due, &appliedMode, fullWindow)
		if err != nil {
			a.epd.Close()
			return err
		}
		if pushed {
			lastBuffer = buf
		}

		select {
		case <-ctx.Done():
			return a.shutdown()
		case err := <-serverErr:
			a.epd.Close()
			return fmt.Errorf("preview server: %w", err)
		case <-ticker.C:
		}
	}
}

// refresh applies the planner's decision for one cycle: it picks a refresh
// waveform based on whether buf differs from the frame on the panel, re-inits
// the controller only when the waveform's LUT changes, and pushes the frame
// (full or partial). appliedMode is updated in place. It reports whether a
// frame was actually pushed (false on a skip), so the caller knows whether to
// advance its last-pushed buffer.
//
// due is the refresh-queue gate: a content change is only allowed to drive a
// refresh when at least one widget is due this minute, so widgets on
// independent cadences coalesce instead of each flashing the panel. The
// planner still issues its periodic full/grayscale refresh regardless of due
// (burn-in protection), and an undue change is simply held until the next due
// cycle rather than dropped.
func (a *App) refresh(buf, lastBuffer []byte, due bool, appliedMode *InitMode, window Region) (bool, error) {
	kind := a.planner.next(due && !bytes.Equal(buf, lastBuffer))
	if kind == refreshSkip {
		return false, nil
	}

	if target := initModeForKind(kind); target != *appliedMode {
		if err := a.epd.Init(target); err != nil {
			return false, fmt.Errorf("init display %q: %w", a.profile.Name, err)
		}
		*appliedMode = target
	}

	if kind == refreshPartial {
		if err := a.epd.DisplayPartial(buf, lastBuffer, window); err != nil {
			return false, fmt.Errorf("display partial: %w", err)
		}
		return true, nil
	}

	if err := a.epd.Display(buf); err != nil {
		return false, fmt.Errorf("display: %w", err)
	}
	return true, nil
}

// initModeForKind maps a planner decision to the init sequence whose waveform
// LUT the panel needs loaded before that refresh.
func initModeForKind(kind refreshKind) InitMode {
	switch kind {
	case refreshFast:
		return InitFast
	case refreshPartial:
		return InitPartial
	case refreshGray:
		return Init4Gray
	default: // refreshFull
		return InitFull
	}
}

// shutdown runs the graceful-exit cleanup: optionally clears the panel to
// white (so a stopped service shows an obviously-blank screen instead of
// a stale dashboard), then runs the sleep sequence and releases the
// hardware. Only the signal-driven shutdown path uses this; render and
// display error paths skip the clear so a partial/broken frame isn't
// "corrected" on top of an already-failing state.
//
// The clear first re-initializes the panel to its full-refresh waveform.
// The render loop only re-inits when the planned waveform changes, so in BW
// mode it settles into partial-window mode (the flicker-free steady state);
// a clear issued in that state won't drive a full-screen refresh and the
// panel would retain its last frame. Re-init (hardware reset + InitFull /
// Init4Gray) restores a full-frame waveform before pushing the white frame.
//
// A re-init or Clear failure is reported but Close still runs — we want the
// panel in deep sleep even if the refresh couldn't complete, otherwise we'd
// leave it drawing power.
func (a *App) shutdown() error {
	var clearErr error
	if a.clearOnShutdown {
		if err := a.epd.Init(initModeFor(a.profile.Color)); err != nil {
			clearErr = fmt.Errorf("clear re-init: %w", err)
		} else {
			clearErr = a.epd.Clear()
		}
	}
	closeErr := a.epd.Close()
	return errors.Join(clearErr, closeErr)
}

// buildDashboard creates a Dashboard from config, instantiating widgets via
// the registry. It validates that widget bounds fit within the display.
func buildDashboard(cfg *Config, profile *DisplayProfile, registry *widget.Registry, deps widget.Deps) (*Dashboard, error) {
	if len(cfg.Dashboard.Screens) == 0 {
		if cfg.Dashboard.RotateInterval > 0 {
			return nil, fmt.Errorf("dashboard.rotate_interval is set but no screens are configured")
		}
		return NewDashboard([]*Screen{NewScreen("default", nil)}, 0, deps.Now), nil
	}

	displayBounds := image.Rect(0, 0, profile.Width, profile.Height)
	var screens []*Screen

	for _, sc := range cfg.Dashboard.Screens {
		var ws []widget.Widget
		var cadences []time.Duration
		for _, wc := range sc.Widgets {
			bounds := image.Rect(wc.Bounds[0], wc.Bounds[1], wc.Bounds[2], wc.Bounds[3])
			if bounds.Empty() {
				return nil, fmt.Errorf("screen %q: widget %q has empty bounds %v",
					sc.Name, wc.Type, wc.Bounds)
			}
			if !bounds.In(displayBounds) {
				return nil, fmt.Errorf("screen %q: widget %q bounds %v exceed display %v",
					sc.Name, wc.Type, bounds, displayBounds)
			}
			w, err := registry.Create(wc.Type, bounds, wc.Config, deps)
			if err != nil {
				return nil, fmt.Errorf("screen %q: widget %q: %w", sc.Name, wc.Type, err)
			}
			ws = append(ws, w)
			cadences = append(cadences, wc.Refresh.cadence())
		}
		screen := NewScreen(sc.Name, ws)
		screen.schedule = refreshSchedule{cadences: cadences}
		screens = append(screens, screen)
	}

	return NewDashboard(screens, time.Duration(cfg.Dashboard.RotateInterval), deps.Now), nil
}

// initModeFor maps a ColorDepth to the init sequence the panel needs
// before any frame data is written. Gray4 requires the Init4Gray
// sequence (different temperature waveform); BW (and any future depth
// without a special init) uses the standard full init.
func initModeFor(c ColorDepth) InitMode {
	if c == Gray4 {
		return Init4Gray
	}
	return InitFull
}

// applyColorMode returns a profile pinned to the color depth requested in
// config. "" or "bw" leaves the base profile untouched; "gray4" returns a
// shallow copy with Color overridden. We copy rather than mutate so the
// shared Profiles map is never modified — each App owns its own profile.
func applyColorMode(base *DisplayProfile, mode string) (*DisplayProfile, error) {
	switch mode {
	case "", "bw":
		return base, nil
	case "gray4":
		if !base.Capabilities.Grayscale {
			return nil, fmt.Errorf("display %q does not support grayscale", base.Name)
		}
		p := *base
		p.Color = Gray4
		return &p, nil
	default:
		return nil, fmt.Errorf("invalid color_mode: %q", mode)
	}
}

// createSPIBackendFn creates the SPI hardware backend. Overridden by
// spi_backend_hardware.go when built with -tags hardware.
var createSPIBackendFn = func(_ *Config, _ *DisplayProfile) (Hardware, error) {
	return nil, fmt.Errorf("spi backend requires building with -tags hardware")
}

// createBackend creates a Hardware backend based on config.
func createBackend(cfg *Config, profile *DisplayProfile) (Hardware, error) {
	switch cfg.Backend {
	case "preview":
		return NewWebPreview(profile), nil
	case "image":
		return NewImageBackend(profile, cfg.Image.OutputDir), nil
	case "spi":
		return createSPIBackendFn(cfg, profile)
	default:
		return nil, fmt.Errorf("unsupported backend: %q", cfg.Backend)
	}
}
