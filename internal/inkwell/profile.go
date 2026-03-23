package inkwell

// DisplayProfile contains everything needed to drive a specific e-ink display.
// This is pure data — no methods, no logic, no interface implementations.
// Adding a new display means filling in a new DisplayProfile, not writing a driver.
type DisplayProfile struct {
	Name   string // e.g. "waveshare_7in5_v2"
	Width  int
	Height int
	Color  ColorDepth

	Capabilities Capabilities

	// Init sequences per mode (nil = mode not supported)
	InitFull    []Command
	InitFast    []Command // nil if Capabilities.FastRefresh is false
	InitPartial []Command // nil if Capabilities.PartialRefresh is false
	Init4Gray   []Command // nil if Capabilities.Grayscale is false

	// Display data commands
	OldBufferCmd byte // 0x10 for most displays, 0x24 for some
	NewBufferCmd byte // 0x13 for most displays, 0x26 for some
	RefreshCmd   byte // 0x12 for all known displays

	// Partial refresh window command (0x90 for most)
	PartialWindowCmd byte
	PartialEnterCmd  byte // 0x91
	PartialVCOM      []byte

	// Sleep sequence
	SleepSequence []Command

	// Waveform LUT (nil if display has built-in LUTs)
	LUT []byte
}

// BufferSize returns the framebuffer size in bytes for this display.
func (p *DisplayProfile) BufferSize() int {
	switch p.Color {
	case Gray4:
		return p.Width * p.Height / 4
	case Color7:
		return p.Width * p.Height / 2
	default: // BW
		return p.Width * p.Height / 8
	}
}

// Profiles maps display names to their profiles for config-driven selection.
var Profiles = map[string]*DisplayProfile{
	"waveshare_7in5_v2": &Waveshare7in5V2,
}

// Waveshare7in5V2 is the profile for the Waveshare 7.5" e-Paper V2 (800x480, BW).
// Init sequences sourced from docs/05-spi-command-reference.md.
var Waveshare7in5V2 = DisplayProfile{
	Name:   "waveshare_7in5_v2",
	Width:  800,
	Height: 480,
	Color:  BW,
	Capabilities: Capabilities{
		FastRefresh:    true,
		PartialRefresh: true,
		Grayscale:      true,
	},
	InitFull: []Command{
		{0x06, []byte{0x17, 0x17, 0x28, 0x17}}, // Booster soft start
		{0x01, []byte{0x07, 0x07, 0x28, 0x17}},  // Power setting
		{0x04, nil},                               // Power on (+ busy wait)
		{0x00, []byte{0x1F}},                      // Panel setting
		{0x61, []byte{0x03, 0x20, 0x01, 0xE0}},   // Resolution 800x480
		{0x15, []byte{0x00}},                      // Dual SPI off
		{0x50, []byte{0x10, 0x07}},                // VCOM interval
		{0x60, []byte{0x22}},                      // TCON setting
	},
	InitFast: []Command{
		{0x00, []byte{0x1F}},                    // Panel setting
		{0x50, []byte{0x10, 0x07}},              // VCOM interval
		{0x04, nil},                             // Power on (+ busy wait)
		{0x06, []byte{0x27, 0x27, 0x18, 0x17}}, // Booster
		{0xE0, []byte{0x02}},                    // Cascade setting
		{0xE5, []byte{0x5A}},                    // Force temperature
	},
	InitPartial: []Command{
		{0x00, []byte{0x1F}}, // Panel setting
		{0x04, nil},          // Power on (+ busy wait)
		{0xE0, []byte{0x02}}, // Cascade setting
		{0xE5, []byte{0x6E}}, // Force temperature
	},
	Init4Gray: []Command{
		{0x00, []byte{0x1F}},
		{0x50, []byte{0x10, 0x07}},
		{0x04, nil},
		{0x06, []byte{0x27, 0x27, 0x18, 0x17}},
		{0xE0, []byte{0x02}},
		{0xE5, []byte{0x5F}},
	},
	OldBufferCmd:     0x10,
	NewBufferCmd:     0x13,
	RefreshCmd:       0x12,
	PartialWindowCmd: 0x90,
	PartialEnterCmd:  0x91,
	PartialVCOM:      []byte{0xA9, 0x07},
	SleepSequence: []Command{
		{0x50, []byte{0xF7}}, // VCOM setting
		{0x02, nil},          // Power off (+ busy wait)
		{0x07, []byte{0xA5}}, // Deep sleep
	},
}
