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

// weatherCacheTTL is how long the shared weather Provider reuses a fetched
// forecast before refetching. The panel refresh cadence is governed separately
// by each widget's top-level refresh config.
const weatherCacheTTL = 3 * time.Hour

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
	// Hand every widget a clock already in the dashboard's display zone.
	// clock, date, fuzzy_clock and weekly-calendar all format whatever
	// time.Time they are given, so zoning here is the single place that
	// decides what the whole panel reads — rather than each widget
	// re-resolving it and drifting apart.
	loc, err := cfg.Location()
	if err != nil {
		return nil, err
	}
	deps := o.deps
	if deps.Now == nil {
		deps.Now = time.Now
	}
	base := deps.Now
	deps.Now = func() time.Time { return base().In(loc) }
	if deps.DataSources == nil {
		deps.DataSources = make(map[string]any)
	}
	if _, ok := deps.DataSources["http_client"]; !ok {
		deps.DataSources["http_client"] = http.DefaultClient
	}
	// Build the shared weather Provider once from the top-level weather config
	// and inject it so every weather widget deduplicates fetches through one
	// cache. Widgets resolve their per-widget overrides against the Provider's
	// defaults. A caller may pre-inject "weather" to override it (e.g. tests).
	if _, ok := deps.DataSources["weather"]; !ok {
		httpClient, _ := deps.DataSources["http_client"].(weather.HTTPClient)
		deps.DataSources["weather"] = weather.NewProvider(
			httpClient, weatherCacheTTL, deps.Now,
			weather.Settings{
				Location: weather.Location{Latitude: cfg.Weather.Latitude, Longitude: cfg.Weather.Longitude},
				Model:    weather.Model(cfg.Weather.Model),
				TempUnit: cfg.Weather.TempUnit,
			},
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
		planner:         newRefreshPlanner(profile.Color, defaultFullEvery),
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

	// lastBuffer is the frame last pushed to the panel, used to detect
	// unchanged content. The panel itself is initialised per push (see
	// refresh), so there is nothing to set up before the first cycle.
	var lastBuffer []byte

	for {
		ws, due := a.nextCycle()

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

		pushed, err := a.refresh(buf, lastBuffer, due)
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

// nextCycle picks the widgets to render this cycle and reports whether the
// cycle is allowed to push a changed frame to the panel.
//
// The refresh queue normally holds a changed frame back until one of the
// screen's widgets is due this minute, so widgets on independent cadences
// coalesce into a single flash (ADR 0011). A screen rotation is not an
// ordinary content change though: it is a deliberate, user-visible switch,
// and Dashboard advances it on its own interval with no relation to any
// widget cadence. Gating it on the same queue quantises the rotation to the
// new screen's cadences, so a 20m rotation against 15m widgets lands up to
// ten minutes late and the lateness varies per cycle. A rotation is
// therefore inherently due.
//
// This does mean a rotation adds a flash the queue would otherwise have
// suppressed — a real cost against the low-flash goal, accepted because a
// rotation nobody asked for is invisible while a rotation that arrives late
// is just wrong.
func (a *App) nextCycle() ([]widget.Widget, bool) {
	screen, rotated := a.dashboard.CurrentScreen()
	if screen == nil {
		return nil, false
	}
	return screen.Widgets(), rotated || screen.AnyDue(a.now())
}

// refresh applies the planner's decision for one cycle: it picks a refresh
// waveform based on whether buf differs from the frame on the panel and, unless
// the decision is to skip, runs the full lifecycle around the push — hardware
// reset plus the waveform's init sequence, the frame, then EPD.Sleep (a settle,
// the sleep sequence, and the deep-sleep settle). It reports whether a frame
// was actually pushed (false on a skip), so the caller knows whether to advance
// its last-pushed buffer.
//
// Every push starts from a fresh init, in either color mode, and ends with the
// panel powered off and in deep sleep, which is what the vendor wiki requires
// of a long-running panel (left in its high-voltage state it is damaged
// irreparably) and what makes a settled image insensitive to light on the TFT
// backplane between refreshes. The timing of the power-off is what matters:
// issued 20 ms after BUSY released it wiped the freshly written image edge to
// edge on real hardware, so EPD.Sleep waits several seconds first. The whole
// cycle costs a few seconds against a push cadence with a one-minute floor.
// See ADR 0012.
//
// due is the refresh-queue gate: a content change is only allowed to drive a
// refresh when at least one widget is due this minute, so widgets on
// independent cadences coalesce instead of each flashing the panel. The
// planner still issues its periodic full/grayscale refresh regardless of due
// (burn-in protection), and an undue change is simply held until the next due
// cycle rather than dropped.
//
// In BW mode every due change drives a full-screen fast refresh (a single
// flash). A windowed partial refresh was tried for flicker-free per-change
// updates, but redrawing changed pixels cleanly needs the force-drive (old=^new)
// trick, which the controller only resolves under the full/fast waveform — a
// partial-windowed force-drive settles the box inverted on real hardware. The
// full-screen fast path reuses the proven Display sequence instead.
func (a *App) refresh(buf, lastBuffer []byte, due bool) (bool, error) {
	kind := a.planner.next(due && !bytes.Equal(buf, lastBuffer))
	if kind == refreshSkip {
		return false, nil
	}

	if err := a.epd.Init(initModeForKind(kind)); err != nil {
		return false, fmt.Errorf("init display %q: %w", a.profile.Name, err)
	}
	if err := a.epd.Display(buf); err != nil {
		return false, fmt.Errorf("display: %w", err)
	}
	if err := a.epd.Sleep(); err != nil {
		return false, fmt.Errorf("sleep display %q: %w", a.profile.Name, err)
	}
	return true, nil
}

// initModeForKind maps a planner decision to the init sequence whose waveform
// LUT the panel needs loaded before that refresh.
func initModeForKind(kind refreshKind) InitMode {
	switch kind {
	case refreshFast:
		return InitFast
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
// The clear first re-initializes the panel: the render loop leaves it in deep
// sleep after every push, and a sleeping controller ignores frame data until
// it has been reset and initialised again. Init (hardware reset + InitFull /
// Init4Gray) wakes it with a full-frame waveform before pushing the white frame.
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
