package inkwell

import (
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

// Config holds application configuration loaded from YAML.
type Config struct {
	Display string        `yaml:"display"`
	Backend string        `yaml:"backend"`
	Preview PreviewConfig `yaml:"preview,omitempty"`
	Image   ImageConfig   `yaml:"image,omitempty"`
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
