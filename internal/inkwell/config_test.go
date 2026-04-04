package inkwell

import (
	"strings"
	"testing"
)

func TestLoadConfig_ValidConfig(t *testing.T) {
	yaml := `
display: waveshare_7in5_v2
backend: image
preview:
  port: 9090
image:
  output_dir: /tmp/frames
`
	cfg, err := LoadConfig(strings.NewReader(yaml))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Display != "waveshare_7in5_v2" {
		t.Errorf("Display = %q, want waveshare_7in5_v2", cfg.Display)
	}
	if cfg.Backend != "image" {
		t.Errorf("Backend = %q, want image", cfg.Backend)
	}
	if cfg.Preview.Port != 9090 {
		t.Errorf("Preview.Port = %d, want 9090", cfg.Preview.Port)
	}
	if cfg.Image.OutputDir != "/tmp/frames" {
		t.Errorf("Image.OutputDir = %q, want /tmp/frames", cfg.Image.OutputDir)
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	cfg, err := LoadConfig(strings.NewReader(""))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Display != "waveshare_7in5_v2" {
		t.Errorf("Display = %q, want waveshare_7in5_v2", cfg.Display)
	}
	if cfg.Backend != "preview" {
		t.Errorf("Backend = %q, want preview", cfg.Backend)
	}
	if cfg.Preview.Port != 8080 {
		t.Errorf("Preview.Port = %d, want 8080", cfg.Preview.Port)
	}
	if cfg.Image.OutputDir != "output" {
		t.Errorf("Image.OutputDir = %q, want output", cfg.Image.OutputDir)
	}
}

func TestLoadConfig_UnknownProfile(t *testing.T) {
	yaml := `display: nonexistent_display`
	_, err := LoadConfig(strings.NewReader(yaml))
	if err == nil {
		t.Fatal("expected error for unknown display profile, got nil")
	}
	if !strings.Contains(err.Error(), "nonexistent_display") {
		t.Errorf("error = %q, want to mention unknown profile name", err.Error())
	}
}

func TestLoadConfig_InvalidBackend(t *testing.T) {
	yaml := `backend: serial`
	_, err := LoadConfig(strings.NewReader(yaml))
	if err == nil {
		t.Fatal("expected error for invalid backend, got nil")
	}
	if !strings.Contains(err.Error(), "serial") {
		t.Errorf("error = %q, want to mention invalid backend", err.Error())
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	_, err := LoadConfig(strings.NewReader("{{{{not yaml"))
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Display != "waveshare_7in5_v2" {
		t.Errorf("Display = %q, want waveshare_7in5_v2", cfg.Display)
	}
	if cfg.Backend != "preview" {
		t.Errorf("Backend = %q, want preview", cfg.Backend)
	}
	if cfg.Preview.Port != 8080 {
		t.Errorf("Preview.Port = %d, want 8080", cfg.Preview.Port)
	}
	if cfg.Image.OutputDir != "output" {
		t.Errorf("Image.OutputDir = %q, want output", cfg.Image.OutputDir)
	}
}
