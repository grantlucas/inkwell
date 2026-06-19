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
          refresh: "1m"
          config:
            format: "15:04"
    - name: detail
      widgets:
        - type: clock
          bounds: [0, 0, 200, 50]
          refresh: "1m"
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
	if cfg.ColorMode != "gray4" {
		t.Errorf("ColorMode = %q, want gray4", cfg.ColorMode)
	}
	if !cfg.ClearOnShutdown {
		t.Errorf("ClearOnShutdown = false, want true (default)")
	}
}

func TestLoadConfig_WidgetRefresh(t *testing.T) {
	base := func(widget string) string {
		return "display: waveshare_7in5_v2\nbackend: preview\n" +
			"dashboard:\n  screens:\n    - name: main\n      widgets:\n" + widget
	}
	cases := []struct {
		label       string
		widget      string
		wantErr     string        // "" means expect success
		wantCadence time.Duration // expected schedule cadence on success
	}{
		{
			label:       "valid duration parses",
			widget:      "        - type: clock\n          bounds: [0, 0, 100, 50]\n          refresh: \"5m\"\n",
			wantCadence: 5 * time.Minute,
		},
		{
			label:       "static never refreshes",
			widget:      "        - type: separator\n          bounds: [0, 0, 100, 50]\n          refresh: \"static\"\n",
			wantCadence: 0,
		},
		{
			label:       "never is an alias for static",
			widget:      "        - type: separator\n          bounds: [0, 0, 100, 50]\n          refresh: \"never\"\n",
			wantCadence: 0,
		},
		{
			label:   "missing refresh is rejected",
			widget:  "        - type: clock\n          bounds: [0, 0, 100, 50]\n",
			wantErr: "refresh is required",
		},
		{
			label:   "sub-minute refresh is rejected",
			widget:  "        - type: clock\n          bounds: [0, 0, 100, 50]\n          refresh: \"30s\"\n",
			wantErr: "whole-minute duration",
		},
		{
			label:   "non-whole-minute refresh is rejected",
			widget:  "        - type: clock\n          bounds: [0, 0, 100, 50]\n          refresh: \"90s\"\n",
			wantErr: "whole-minute duration",
		},
		{
			label:   "garbage refresh is rejected",
			widget:  "        - type: clock\n          bounds: [0, 0, 100, 50]\n          refresh: \"soon\"\n",
			wantErr: "invalid refresh",
		},
		{
			label:   "non-string refresh is rejected",
			widget:  "        - type: clock\n          bounds: [0, 0, 100, 50]\n          refresh:\n            nested: true\n",
			wantErr: "parse config",
		},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			cfg, err := LoadConfig(strings.NewReader(base(tc.widget)))
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error mentioning %q, got nil", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("error = %q, want mention of %q", err.Error(), tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("LoadConfig: %v", err)
			}
			if got := cfg.Dashboard.Screens[0].Widgets[0].Refresh.cadence(); got != tc.wantCadence {
				t.Errorf("widget refresh cadence = %v, want %v", got, tc.wantCadence)
			}
		})
	}
}

func TestLoadConfig_ClearOnShutdown(t *testing.T) {
	cases := []struct {
		label string
		yaml  string
		want  bool
	}{
		{
			label: "defaults to true when omitted",
			yaml:  "display: waveshare_7in5_v2",
			want:  true,
		},
		{
			label: "explicit true",
			yaml:  "display: waveshare_7in5_v2\nclear_on_shutdown: true",
			want:  true,
		},
		{
			label: "explicit false opts out",
			yaml:  "display: waveshare_7in5_v2\nclear_on_shutdown: false",
			want:  false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			cfg, err := LoadConfig(strings.NewReader(tc.yaml))
			if err != nil {
				t.Fatalf("LoadConfig: %v", err)
			}
			if cfg.ClearOnShutdown != tc.want {
				t.Errorf("ClearOnShutdown = %v, want %v", cfg.ClearOnShutdown, tc.want)
			}
		})
	}
}

func TestLoadConfig_ColorMode(t *testing.T) {
	cases := []struct {
		label   string
		yaml    string
		want    string
		wantErr string
	}{
		{
			label: "default omitted",
			yaml:  "display: waveshare_7in5_v2",
			want:  "gray4",
		},
		{
			label: "explicit bw",
			yaml:  "display: waveshare_7in5_v2\ncolor_mode: bw",
			want:  "bw",
		},
		{
			label: "explicit gray4",
			yaml:  "display: waveshare_7in5_v2\ncolor_mode: gray4",
			want:  "gray4",
		},
		{
			label:   "rejects unknown",
			yaml:    "display: waveshare_7in5_v2\ncolor_mode: color7",
			wantErr: "color_mode",
		},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			cfg, err := LoadConfig(strings.NewReader(tc.yaml))
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error mentioning %q, got nil", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("error = %q, want mention of %q", err.Error(), tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("LoadConfig: %v", err)
			}
			if cfg.ColorMode != tc.want {
				t.Errorf("ColorMode = %q, want %q", cfg.ColorMode, tc.want)
			}
		})
	}
}
