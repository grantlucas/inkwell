package inkwell

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

// HTTPServer is implemented by Hardware backends that also serve HTTP.
// App.Run starts an http.Server when the backend satisfies this interface.
type HTTPServer interface {
	Mux() *http.ServeMux
}

// App wires together config, backend, EPD, and compositor into a running
// application. Use NewApp to construct, then Run to start the render loop.
type App struct {
	hw         Hardware
	epd        *EPD
	comp       *Compositor
	profile    *DisplayProfile
	interval   time.Duration
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
}

// WithHardware injects a Hardware backend, overriding config-driven selection.
func WithHardware(hw Hardware) AppOption {
	return func(o *appOptions) { o.hw = hw }
}

// WithInterval sets the render loop sleep interval.
func WithInterval(d time.Duration) AppOption {
	return func(o *appOptions) { o.interval = d }
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

	epd := NewEPD(hw, profile)
	comp := NewCompositor(profile)

	return &App{
		hw:         hw,
		epd:        epd,
		comp:       comp,
		profile:    profile,
		interval:   o.interval,
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

		srv := &http.Server{Handler: hs.Mux()}
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
		frame, err := a.comp.Render()
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

// createBackend creates a Hardware backend based on config.
func createBackend(cfg *Config, profile *DisplayProfile) (Hardware, error) {
	switch cfg.Backend {
	case "preview":
		return NewWebPreview(profile), nil
	case "image":
		return NewImageBackend(profile, cfg.Image.OutputDir), nil
	default:
		return nil, fmt.Errorf("unsupported backend: %q", cfg.Backend)
	}
}
