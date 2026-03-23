package inkwell

import "testing"

func TestBufferSize(t *testing.T) {
	tests := []struct {
		name  string
		prof  DisplayProfile
		want  int
	}{
		{
			name: "BW 800x480",
			prof: DisplayProfile{Width: 800, Height: 480, Color: BW},
			want: 800 * 480 / 8, // 48000
		},
		{
			name: "Gray4 800x480",
			prof: DisplayProfile{Width: 800, Height: 480, Color: Gray4},
			want: 800 * 480 / 4, // 96000
		},
		{
			name: "Color7 800x480",
			prof: DisplayProfile{Width: 800, Height: 480, Color: Color7},
			want: 800 * 480 / 2, // 192000
		},
		{
			name: "BW small 16x16",
			prof: DisplayProfile{Width: 16, Height: 16, Color: BW},
			want: 16 * 16 / 8, // 32
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.prof.BufferSize(); got != tt.want {
				t.Errorf("BufferSize() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestProfilesLookup(t *testing.T) {
	// Hit: known profile exists
	prof, ok := Profiles["waveshare_7in5_v2"]
	if !ok {
		t.Fatal("Profiles[\"waveshare_7in5_v2\"] not found")
	}
	if prof.Width != 800 || prof.Height != 480 {
		t.Errorf("profile resolution = %dx%d, want 800x480", prof.Width, prof.Height)
	}
	if prof.Color != BW {
		t.Errorf("profile color = %v, want BW", prof.Color)
	}
	if prof.Name != "waveshare_7in5_v2" {
		t.Errorf("profile name = %q, want %q", prof.Name, "waveshare_7in5_v2")
	}

	// Miss: unknown profile
	_, ok = Profiles["nonexistent"]
	if ok {
		t.Error("Profiles[\"nonexistent\"] should not exist")
	}
}

func TestWaveshare7in5V2Profile(t *testing.T) {
	p := &Waveshare7in5V2

	// Buffer size for BW
	if got := p.BufferSize(); got != 48000 {
		t.Errorf("BufferSize() = %d, want 48000", got)
	}

	// Capabilities
	if !p.Capabilities.FastRefresh {
		t.Error("expected FastRefresh")
	}
	if !p.Capabilities.PartialRefresh {
		t.Error("expected PartialRefresh")
	}
	if !p.Capabilities.Grayscale {
		t.Error("expected Grayscale")
	}

	// Init sequences should not be nil
	if p.InitFull == nil {
		t.Error("InitFull should not be nil")
	}
	if p.InitFast == nil {
		t.Error("InitFast should not be nil")
	}
	if p.InitPartial == nil {
		t.Error("InitPartial should not be nil")
	}
	if p.Init4Gray == nil {
		t.Error("Init4Gray should not be nil")
	}

	// Display commands match SPI reference
	if p.OldBufferCmd != 0x10 {
		t.Errorf("OldBufferCmd = %#x, want 0x10", p.OldBufferCmd)
	}
	if p.NewBufferCmd != 0x13 {
		t.Errorf("NewBufferCmd = %#x, want 0x13", p.NewBufferCmd)
	}
	if p.RefreshCmd != 0x12 {
		t.Errorf("RefreshCmd = %#x, want 0x12", p.RefreshCmd)
	}

	// Sleep sequence
	if p.SleepSequence == nil {
		t.Error("SleepSequence should not be nil")
	}
	if len(p.SleepSequence) != 3 {
		t.Errorf("SleepSequence length = %d, want 3", len(p.SleepSequence))
	}

	// First init command should be booster soft start (0x06)
	if p.InitFull[0].Reg != 0x06 {
		t.Errorf("InitFull[0].Reg = %#x, want 0x06", p.InitFull[0].Reg)
	}
}
