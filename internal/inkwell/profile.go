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
	// InitFull is the vendor init() with one deliberate difference, kept on
	// hardware evidence: it does not send the power setting (0x01) and it uses
	// the fast sequence's booster bytes.
	//
	// Vendor init() sends 0x01 = {07 07 28 17}: VGH/VGL ±20 V, VDH ≈ +10.5 V,
	// VDL = −7 V (Waveshare lowered these from {07 07 3F 3F}, ±15 V, in Sept
	// 2024 for a newer film batch). Omitting the command leaves the UC8179's
	// power-on defaults: VG_LVL the same ±20 V, VDH/VDL = 0x3A = ±14 V — a
	// harder, symmetric source drive than the vendor's bytes. On the panel this
	// project runs, the vendor bytes were tried (2026-09-19) and the full
	// refresh came back visibly lighter than with the defaults; the defaults
	// hold. Booster 0x06: bits [5:3] are drive strength (1–8). Vendor init()
	// uses {17 17 28 17} (start-up phases at 3, sustain phase C1 at 6); this
	// profile keeps the fast/4-gray {27 27 18 17} (start-up 5, sustain 4),
	// which is the combination that held. ADR 0009 first made this change for
	// a reason the datasheet does not support (it thought the default was
	// "lower"); ADR 0013 corrects the rationale and keeps the bytes.
	//
	// Do not tune these by eye on the preview, which cannot show drive
	// strength; any change needs the on-device photo protocol in ADR 0014.
	InitFull: []Command{
		{0x06, []byte{0x27, 0x27, 0x18, 0x17}}, // Booster soft start (fast-path strength)
		{0x04, nil},                            // Power on (+ busy wait) — power setting stays at reset default
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
