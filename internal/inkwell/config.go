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
type WidgetConfig struct {
	Type   string         `yaml:"type"`
	Bounds [4]int         `yaml:"bounds"`
	Config map[string]any `yaml:"config"`
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
	Refresh         RefreshConfig   `yaml:"refresh,omitempty"`
}

// RefreshConfig tunes the refresh-mode cadence (see refreshPlanner). The
// values count render cycles, so at the default 60s interval FullEvery: 60
// is roughly hourly. Only bw mode uses fast/partial refreshes; gray4
// honors FullEvery for its periodic forced refresh and otherwise refreshes
// on content change.
type RefreshConfig struct {
	FullEvery int `yaml:"full_every"` // cycles between full / forced grayscale refreshes
	FastEvery int `yaml:"fast_every"` // cycles between fast refreshes (bw only; 0 = never)
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
		Refresh:         RefreshConfig{FullEvery: 60, FastEvery: 10},
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

	if cfg.Refresh.FullEvery < 0 || cfg.Refresh.FastEvery < 0 {
		return nil, fmt.Errorf("refresh cadence must be non-negative")
	}

	return cfg, nil
}
