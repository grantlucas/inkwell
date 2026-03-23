package inkwell

import "testing"

func TestColorDepthString(t *testing.T) {
	tests := []struct {
		depth ColorDepth
		want  string
	}{
		{BW, "BW"},
		{Gray4, "Gray4"},
		{Color7, "Color7"},
		{ColorDepth(99), "ColorDepth(99)"},
	}
	for _, tt := range tests {
		if got := tt.depth.String(); got != tt.want {
			t.Errorf("ColorDepth(%d).String() = %q, want %q", int(tt.depth), got, tt.want)
		}
	}
}

func TestInitModeString(t *testing.T) {
	tests := []struct {
		mode InitMode
		want string
	}{
		{InitFull, "InitFull"},
		{InitFast, "InitFast"},
		{InitPartial, "InitPartial"},
		{Init4Gray, "Init4Gray"},
		{InitMode(99), "InitMode(99)"},
	}
	for _, tt := range tests {
		if got := tt.mode.String(); got != tt.want {
			t.Errorf("InitMode(%d).String() = %q, want %q", int(tt.mode), got, tt.want)
		}
	}
}

func TestCommandStruct(t *testing.T) {
	cmd := Command{Reg: 0x04, Data: nil}
	if cmd.Reg != 0x04 {
		t.Errorf("Command.Reg = %#x, want 0x04", cmd.Reg)
	}
	if cmd.Data != nil {
		t.Errorf("Command.Data = %v, want nil", cmd.Data)
	}

	cmd2 := Command{Reg: 0x06, Data: []byte{0x17, 0x17}}
	if len(cmd2.Data) != 2 {
		t.Errorf("Command.Data length = %d, want 2", len(cmd2.Data))
	}
}

func TestCapabilitiesStruct(t *testing.T) {
	caps := Capabilities{
		FastRefresh:    true,
		PartialRefresh: false,
		Grayscale:      true,
	}
	if !caps.FastRefresh {
		t.Error("expected FastRefresh to be true")
	}
	if caps.PartialRefresh {
		t.Error("expected PartialRefresh to be false")
	}
	if !caps.Grayscale {
		t.Error("expected Grayscale to be true")
	}
}
