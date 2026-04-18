package inkwell

import (
	"context"
	"errors"
	"fmt"
	"image"
	"net"
	"net/http"
	"sync"
	"time"

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
	interval        time.Duration
	listenAddr      string
	listener        net.Listener
	ready           chan struct{}
	shutdownTimeout time.Duration
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

	profile, ok := Profiles[cfg.Display]
	if !ok {
		return nil, fmt.Errorf("unknown display profile: %q", cfg.Display)
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
		interval:        o.interval,
		listenAddr:      fmt.Sprintf(":%d", cfg.Preview.Port),
		ready:           make(chan struct{}),
		shutdownTimeout: 5 * time.Second,
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

	if err := a.epd.Init(InitFull); err != nil {
		a.epd.Close()
		return fmt.Errorf("init display: %w", err)
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

	for {
		var ws []widget.Widget
		if screen := a.dashboard.CurrentScreen(); screen != nil {
			ws = screen.Widgets()
		}

		frame, err := a.comp.Render(ws)
		if err != nil {
			a.epd.Close()
			return fmt.Errorf("render: %w", err)
		}

		buf, err := PackImage(a.profile, frame)
		if err != nil {
			a.epd.Close()
			return fmt.Errorf("pack image: %w", err)
		}

		if err := a.epd.Display(buf); err != nil {
			a.epd.Close()
			return fmt.Errorf("display: %w", err)
		}

		select {
		case <-ctx.Done():
			return a.epd.Close()
		case err := <-serverErr:
			a.epd.Close()
			return fmt.Errorf("preview server: %w", err)
		case <-ticker.C:
		}
	}
}

// buildDashboard creates a Dashboard from config, instantiating widgets via
// the registry. It validates that widget bounds fit within the display.
func buildDashboard(cfg *Config, profile *DisplayProfile, registry *widget.Registry, deps widget.Deps) (*Dashboard, error) {
	if len(cfg.Dashboard.Screens) == 0 {
		return NewDashboard([]*Screen{NewScreen("default", nil)}, 0, deps.Now), nil
	}

	displayBounds := image.Rect(0, 0, profile.Width, profile.Height)
	var screens []*Screen

	for _, sc := range cfg.Dashboard.Screens {
		var ws []widget.Widget
		for _, wc := range sc.Widgets {
			bounds := image.Rect(wc.Bounds[0], wc.Bounds[1], wc.Bounds[2], wc.Bounds[3])
			if !bounds.In(displayBounds) {
				return nil, fmt.Errorf("screen %q: widget %q bounds %v exceed display %v",
					sc.Name, wc.Type, bounds, displayBounds)
			}
			w, err := registry.Create(wc.Type, bounds, wc.Config, deps)
			if err != nil {
				return nil, fmt.Errorf("screen %q: widget %q: %w", sc.Name, wc.Type, err)
			}
			ws = append(ws, w)
		}
		screens = append(screens, NewScreen(sc.Name, ws))
	}

	return NewDashboard(screens, time.Duration(cfg.Dashboard.RotateInterval), deps.Now), nil
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
