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
// Init sequences follow the Waveshare reference driver (epd7in5_V2.py); this
// file is their only copy in the tree, and TestInitSendsResetThenProfileCommands
// pins every byte.
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
	// InitFull is the vendor init() byte for byte (epd7in5_V2.py, Waveshare
	// e-Paper master, 2024-10). Two of its bytes are the panel's drive
	// contract and are easy to get wrong, so what they do is recorded here
	// (UC8179c datasheet, R01H / R06H):
	//
	//   - 0x01 power setting {07 07 28 17}: VGH/VGL ±20 V (byte 2 = 0x07),
	//     VDH ≈ +10.5 V (0x28), VDL = −7 V (0x17). Waveshare lowered these
	//     from the older {07 07 3F 3F} (±15 V) in Sept 2024 to suit the newer
	//     film batch. Omitting the command is NOT neutral: the controller's
	//     power-on default is 0x3A = ±14 V on both rails, i.e. a harder,
	//     symmetric drive than the vendor asks for on this film.
	//   - 0x06 booster {17 17 28 17}: bits [5:3] are drive strength (1–8).
	//     Phases A/B at 3 shape the start-up ramp; phase C1 at 6 (0x28) is
	//     the sustain phase that holds the rails for the whole multi-flash
	//     refresh. The fast/4-gray sequences use {27 27 18 17} — a faster
	//     ramp but a weaker sustain (4) — which is fine for their short
	//     waveforms and wrong for the full one.
	//
	// An earlier revision dropped 0x01 and borrowed the fast booster on the
	// theory that the reset default was crisper (ADR 0009). The datasheet
	// says otherwise and the fading it was chasing turned out to be light on
	// the TFT backplane (see ADR 0012), so the profile is back on the vendor
	// sequence. If Waveshare changes init() again, change this to match; do
	// not tune it by eye on the preview, which cannot show drive strength.
	InitFull: []Command{
		{0x06, []byte{0x17, 0x17, 0x28, 0x17}}, // Booster soft start
		{0x01, []byte{0x07, 0x07, 0x28, 0x17}}, // Power setting
		{0x04, nil},                            // Power on (+ busy wait)
		{0x00, []byte{0x1F}},                   // Panel setting
		{0x61, []byte{0x03, 0x20, 0x01, 0xE0}}, // Resolution 800x480
		{0x15, []byte{0x00}},                   // Dual SPI off
		{0x50, []byte{0x10, 0x07}},             // VCOM interval
		{0x60, []byte{0x22}},                   // TCON setting
	},
	InitFast: []Command{
		{0x00, []byte{0x1F}},                   // Panel setting
		{0x50, []byte{0x10, 0x07}},             // VCOM interval
		{0x04, nil},                            // Power on (+ busy wait)
		{0x06, []byte{0x27, 0x27, 0x18, 0x17}}, // Booster
		{0xE0, []byte{0x02}},                   // Cascade setting
		{0xE5, []byte{0x5A}},                   // Force temperature
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
