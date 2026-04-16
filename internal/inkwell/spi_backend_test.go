//go:build !hardware

package inkwell

import (
	"strings"
	"testing"
)

func TestCreateBackend_SPI_NoHardwareTag(t *testing.T) {
	cfg := &Config{Backend: "spi"}
	profile := &Waveshare7in5V2
	_, err := createBackend(cfg, profile)
	if err == nil {
		t.Fatal("expected error for spi backend without hardware tag")
	}
	if !strings.Contains(err.Error(), "requires building with -tags hardware") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewApp_SPIBackendNoHardwareTag(t *testing.T) {
	cfg, err := LoadConfig(strings.NewReader(`
display: waveshare_7in5_v2
backend: spi
`))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	_, err = NewApp(cfg)
	if err == nil {
		t.Fatal("expected error for spi backend without hardware tag")
	}
	if !strings.Contains(err.Error(), "requires building with -tags hardware") {
		t.Fatalf("unexpected error: %v", err)
	}
}
