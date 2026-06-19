package inkwell

import (
	"fmt"
	"io"
	"time"

	"gopkg.in/yaml.v3"
)

// Duration wraps time.Duration for YAML unmarshaling from strings like "5m".
type Duration time.Duration

// UnmarshalYAML parses a duration string (e.g. "5m", "30s") into a Duration.
func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}
	*d = Duration(parsed)
	return nil
}

// WidgetRefresh is a widget's required refresh setting: either a cadence
// duration (how often the widget may refresh the panel, >= 1m) or the literal
// "static" (equivalently "never"), marking a widget whose content never
// changes so it should never trigger a refresh on its own.
type WidgetRefresh struct {
	set    bool          // whether the field was present in the config
	static bool          // "static"/"never": never refresh
	every  time.Duration // cadence when not static
}

// UnmarshalYAML parses a refresh value: the strings "static"/"never", or a
// duration like "5m"/"24h".
func (r *WidgetRefresh) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	r.set = true
	switch s {
	case "static", "never":
		r.static = true
		return nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid refresh %q: %w", s, err)
	}
	r.every = d
	return nil
}

// cadence returns the schedule cadence for this setting: 0 for a static widget
// (never due), otherwise the configured duration.
func (r WidgetRefresh) cadence() time.Duration {
	if r.static {
		return 0
	}
	return r.every
}

// DashboardConfig defines the screen collection and rotation behavior.
type DashboardConfig struct {
	RotateInterval Duration       `yaml:"rotate_interval"`
	Screens        []ScreenConfig `yaml:"screens"`
}

// ScreenConfig defines a named screen with its widget layout.
type ScreenConfig struct {
	Name    string         `yaml:"name"`
	Widgets []WidgetConfig `yaml:"widgets"`
}

// WidgetConfig defines a single widget placement and configuration.
//
// Refresh is required: it sets how often this widget may trigger a panel
// refresh (its render cadence), as a duration of at least one minute (e.g.
// "5m", "24h"), or the literal "static" for a widget that never changes. It is
// the only refresh setting in the config and is fed into the refresh queue,
// which aligns cadences to wall-clock boundaries so widgets sharing a cadence
// coalesce. It is distinct from any widget-specific data-refresh setting nested
// under Config (e.g. the weekly-calendar's config.refresh, which is its data
// cache TTL): Refresh controls when the screen is refreshed, not when the
// widget refetches data.
type WidgetConfig struct {
	Type    string         `yaml:"type"`
	Bounds  [4]int         `yaml:"bounds"`
	Refresh WidgetRefresh  `yaml:"refresh"`
	Config  map[string]any `yaml:"config"`
}

// Config holds application configuration loaded from YAML.
type Config struct {
	Display         string          `yaml:"display"`
	Backend         string          `yaml:"backend"`
	ColorMode       string          `yaml:"color_mode,omitempty"`
	ClearOnShutdown bool            `yaml:"clear_on_shutdown"`
	Preview         PreviewConfig   `yaml:"preview,omitempty"`
	Image           ImageConfig     `yaml:"image,omitempty"`
	Dashboard       DashboardConfig `yaml:"dashboard,omitempty"`
}

// PreviewConfig holds web preview server settings.
type PreviewConfig struct {
	Port int `yaml:"port"`
}

// ImageConfig holds image backend settings.
type ImageConfig struct {
	OutputDir string `yaml:"output_dir"`
}

// DefaultConfig returns a Config with all defaults applied.
//
// ColorMode defaults to "gray4" because the post-refresh rendering reads
// noticeably better on hardware than 1-bit BW — precip bars stay as a
// dark gray instead of collapsing to solid black, and the design intent
// shines through. BW is still available for users who want the smaller
// framebuffer / faster refresh trade-off.
func DefaultConfig() *Config {
	return &Config{
		Display:         "waveshare_7in5_v2",
		Backend:         "preview",
		ColorMode:       "gray4",
		ClearOnShutdown: true,
		Preview:         PreviewConfig{Port: 8080},
		Image:           ImageConfig{OutputDir: "output"},
	}
}

// LoadConfig reads and parses a YAML config from r. It applies defaults for
// missing fields and validates that the display profile exists and the backend
// is supported.
func LoadConfig(r io.Reader) (*Config, error) {
	cfg := DefaultConfig()
	if err := yaml.NewDecoder(r).Decode(cfg); err != nil && err != io.EOF {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if _, ok := Profiles[cfg.Display]; !ok {
		return nil, fmt.Errorf("unknown display profile: %q", cfg.Display)
	}

	switch cfg.Backend {
	case "preview", "image", "spi":
		// valid
	default:
		return nil, fmt.Errorf("invalid backend: %q (must be preview, image, or spi)", cfg.Backend)
	}

	switch cfg.ColorMode {
	case "bw", "gray4":
		// valid
	default:
		return nil, fmt.Errorf("invalid color_mode: %q (must be bw or gray4)", cfg.ColorMode)
	}

	if cfg.Dashboard.RotateInterval < 0 {
		return nil, fmt.Errorf("dashboard.rotate_interval must be non-negative")
	}

	for _, sc := range cfg.Dashboard.Screens {
		for _, wc := range sc.Widgets {
			if !wc.Refresh.set {
				return nil, fmt.Errorf("widget %q: refresh is required (e.g. refresh: \"5m\", or \"static\")", wc.Type)
			}
			if !wc.Refresh.static && (wc.Refresh.every < time.Minute || wc.Refresh.every%time.Minute != 0) {
				return nil, fmt.Errorf("widget %q: refresh must be a whole-minute duration >= 1m or \"static\", got %v", wc.Type, wc.Refresh.every)
			}
		}
	}

	return cfg, nil
}
