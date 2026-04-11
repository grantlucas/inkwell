package inkwell

import (
	"context"
	"fmt"
	"time"
)

// App wires together config, backend, EPD, and compositor into a running
// application. Use NewApp to construct, then Run to start the render loop.
type App struct {
	hw       Hardware
	epd      *EPD
	comp     *Compositor
	profile  *DisplayProfile
	interval time.Duration
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
		hw:       hw,
		epd:      epd,
		comp:     comp,
		profile:  profile,
		interval: o.interval,
	}, nil
}

// Run initializes the display, enters the render loop, and shuts down when
// ctx is cancelled. The loop: compose → pack → display → sleep.
func (a *App) Run(ctx context.Context) error {
	if err := a.epd.Init(InitFull); err != nil {
		return fmt.Errorf("init display: %w", err)
	}

	for {
		frame, err := a.comp.Render()
		if err != nil {
			a.epd.Close()
			return fmt.Errorf("render: %w", err)
		}

		// PackImage cannot fail here: the compositor already validated the
		// color depth during Render, and PackImage supports all color depths
		// that the compositor accepts.
		buf, _ := PackImage(a.profile, frame)

		if err := a.epd.Display(buf); err != nil {
			a.epd.Close()
			return fmt.Errorf("display: %w", err)
		}

		select {
		case <-ctx.Done():
			return a.epd.Close()
		case <-time.After(a.interval):
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
