package inkwell

import (
	"strings"
	"testing"
	"time"
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

func TestLoadConfig_DashboardSection(t *testing.T) {
	input := `
display: waveshare_7in5_v2
backend: preview
dashboard:
  rotate_interval: 5m
  screens:
    - name: main
      widgets:
        - type: clock
          bounds: [300, 200, 500, 260]
          config:
            format: "15:04"
    - name: detail
      widgets:
        - type: clock
          bounds: [0, 0, 200, 50]
`
	cfg, err := LoadConfig(strings.NewReader(input))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if cfg.Dashboard.RotateInterval != Duration(5*time.Minute) {
		t.Errorf("RotateInterval = %v, want 5m", time.Duration(cfg.Dashboard.RotateInterval))
	}
	if len(cfg.Dashboard.Screens) != 2 {
		t.Fatalf("len(Screens) = %d, want 2", len(cfg.Dashboard.Screens))
	}

	s := cfg.Dashboard.Screens[0]
	if s.Name != "main" {
		t.Errorf("Screens[0].Name = %q, want main", s.Name)
	}
	if len(s.Widgets) != 1 {
		t.Fatalf("len(Screens[0].Widgets) = %d, want 1", len(s.Widgets))
	}

	w := s.Widgets[0]
	if w.Type != "clock" {
		t.Errorf("Type = %q, want clock", w.Type)
	}
	if w.Bounds != [4]int{300, 200, 500, 260} {
		t.Errorf("Bounds = %v, want [300 200 500 260]", w.Bounds)
	}
	if w.Config["format"] != "15:04" {
		t.Errorf("Config[format] = %v, want 15:04", w.Config["format"])
	}
}

func TestLoadConfig_NoDashboardDefaults(t *testing.T) {
	cfg, err := LoadConfig(strings.NewReader(""))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if len(cfg.Dashboard.Screens) != 0 {
		t.Errorf("expected empty screens, got %d", len(cfg.Dashboard.Screens))
	}
	if cfg.Dashboard.RotateInterval != 0 {
		t.Errorf("expected zero rotate interval, got %v", cfg.Dashboard.RotateInterval)
	}
}

func TestDuration_UnmarshalYAML(t *testing.T) {
	input := `
display: waveshare_7in5_v2
backend: preview
dashboard:
  rotate_interval: 30s
`
	cfg, err := LoadConfig(strings.NewReader(input))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Dashboard.RotateInterval != Duration(30*time.Second) {
		t.Errorf("RotateInterval = %v, want 30s", time.Duration(cfg.Dashboard.RotateInterval))
	}
}

func TestDuration_NonStringValue(t *testing.T) {
	input := `
display: waveshare_7in5_v2
backend: preview
dashboard:
  rotate_interval:
    nested: object
`
	_, err := LoadConfig(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for non-string duration")
	}
}

func TestDuration_InvalidValue(t *testing.T) {
	input := `
display: waveshare_7in5_v2
backend: preview
dashboard:
  rotate_interval: not-a-duration
`
	_, err := LoadConfig(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for invalid duration")
	}
}

func TestDuration_NegativeRotateInterval(t *testing.T) {
	input := `
display: waveshare_7in5_v2
backend: preview
dashboard:
  rotate_interval: -5m
`
	_, err := LoadConfig(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for negative rotate interval")
	}
	if !strings.Contains(err.Error(), "non-negative") {
		t.Errorf("error = %q, want mention of non-negative", err.Error())
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
