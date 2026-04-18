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
	Display   string          `yaml:"display"`
	Backend   string          `yaml:"backend"`
	Preview   PreviewConfig   `yaml:"preview,omitempty"`
	Image     ImageConfig     `yaml:"image,omitempty"`
	Dashboard DashboardConfig `yaml:"dashboard,omitempty"`
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
func DefaultConfig() *Config {
	return &Config{
		Display: "waveshare_7in5_v2",
		Backend: "preview",
		Preview: PreviewConfig{Port: 8080},
		Image:   ImageConfig{OutputDir: "output"},
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

	return cfg, nil
}
