package inkwell

import (
	"bytes"
	"errors"
	"testing"
)

// --- Test helpers ---

// errorHardware returns an error on the Nth SendCommand call.
type errorHardware struct {
	MockHardware
	failOnCall int
	callCount  int
}

func (e *errorHardware) SendCommand(cmd byte) error {
	e.callCount++
	if e.callCount == e.failOnCall {
		return errors.New("SPI write failed")
	}
	return e.MockHardware.SendCommand(cmd)
}

// errorDataHardware always returns an error on SendData.
type errorDataHardware struct {
	MockHardware
}

func (e *errorDataHardware) SendData(data []byte) error {
	return errors.New("SPI data write failed")
}

// errorDataNthHardware returns an error on the Nth SendData call.
type errorDataNthHardware struct {
	MockHardware
	failOnCall int
	callCount  int
}

func (e *errorDataNthHardware) SendData(data []byte) error {
	e.callCount++
	if e.callCount == e.failOnCall {
		return errors.New("SPI data write failed")
	}
	return e.MockHardware.SendData(data)
}

// errorResetHardware always returns an error on Reset.
type errorResetHardware struct {
	MockHardware
}

func (e *errorResetHardware) Reset() error {
	return errors.New("reset failed")
}

// smallTestProfile returns a minimal 16x16 BW profile for display tests.
func smallTestProfile() *DisplayProfile {
	return &DisplayProfile{
		Name:         "test",
		Width:        16,
		Height:       16,
		Color:        BW,
		OldBufferCmd: 0x10,
		NewBufferCmd: 0x13,
		RefreshCmd:   0x12,
	}
}

// gray4DisplayProfile returns a small Gray4 profile sized for hand-verifiable
// plane-split tests: 16x2 → 32 pixels → 8 bytes 2bpp → 4 bytes per plane.
func gray4DisplayProfile() *DisplayProfile {
	return &DisplayProfile{
		Name:         "test-gray4",
		Width:        16,
		Height:       2,
		Color:        Gray4,
		OldBufferCmd: 0x10,
		NewBufferCmd: 0x13,
		RefreshCmd:   0x12,
	}
}

// partialTestProfile returns a profile that supports partial refresh.
func partialTestProfile() *DisplayProfile {
	return &DisplayProfile{
		Name:             "test",
		Width:            800,
		Height:           480,
		Color:            BW,
		NewBufferCmd:     0x13,
		RefreshCmd:       0x12,
		PartialWindowCmd: 0x90,
		PartialEnterCmd:  0x91,
		PartialVCOM:      []byte{0xA9, 0x07},
		Capabilities:     Capabilities{PartialRefresh: true},
	}
}

// windowDataFromCalls finds the 9-byte partial window data sent after
// the PartialWindowCmd (0x90) in the recorded call sequence.
func windowDataFromCalls(calls []Call) []byte {
	for i, c := range calls {
		if c.Type != "command" || c.Data[0] != 0x90 {
			continue
		}
		if i+1 < len(calls) && calls[i+1].Type == "data" {
			return calls[i+1].Data
		}
	}
	return nil
}

// --- NewEPD ---

func TestNewEPD(t *testing.T) {
	m := &MockHardware{}
	epd := NewEPD(m, &Waveshare7in5V2)
	if epd.Width() != 800 {
		t.Errorf("Width() = %d, want 800", epd.Width())
	}
	if epd.Height() != 480 {
		t.Errorf("Height() = %d, want 480", epd.Height())
	}
}

// --- execSequence ---

func TestExecSequenceThreeCommands(t *testing.T) {
	m := &MockHardware{}
	epd := NewEPD(m, &DisplayProfile{Name: "test", Width: 8, Height: 8, Color: BW})

	seq := []Command{
		{0x06, []byte{0x17, 0x17}}, // command with data
		{0x04, nil},                 // command without data (triggers busy wait)
		{0x00, []byte{0x1F}},       // command with data
	}

	if err := epd.execSequence(seq); err != nil {
		t.Fatal(err)
	}

	expected := []struct {
		typ  string
		data []byte
	}{
		{"command", []byte{0x06}},
		{"data", []byte{0x17, 0x17}},
		{"command", []byte{0x04}},
		{"busy", nil},
		{"command", []byte{0x00}},
		{"data", []byte{0x1F}},
	}

	if len(m.Calls) != len(expected) {
		t.Fatalf("len(Calls) = %d, want %d", len(m.Calls), len(expected))
	}
	for i, exp := range expected {
		got := m.Calls[i]
		if got.Type != exp.typ {
			t.Errorf("Calls[%d].Type = %q, want %q", i, got.Type, exp.typ)
		}
		if exp.data != nil && !bytes.Equal(got.Data, exp.data) {
			t.Errorf("Calls[%d].Data = %v, want %v", i, got.Data, exp.data)
		}
	}
}

func TestExecSequenceErrorPropagation(t *testing.T) {
	eh := &errorHardware{failOnCall: 2}
	epd := NewEPD(eh, &DisplayProfile{Name: "test", Width: 8, Height: 8, Color: BW})

	seq := []Command{
		{0x06, []byte{0x17}},
		{0x04, nil},
		{0x00, []byte{0x1F}},
	}

	err := epd.execSequence(seq)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestExecSequenceSendDataError(t *testing.T) {
	eh := &errorDataHardware{}
	epd := NewEPD(eh, &DisplayProfile{Name: "test", Width: 8, Height: 8, Color: BW})

	err := epd.execSequence([]Command{{0x06, []byte{0x17}}})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- Init ---

func TestInitSendsResetThenProfileCommands(t *testing.T) {
	tests := []struct {
		name     string
		mode     InitMode
		wantCmds []byte
	}{
		{"InitFull", InitFull, []byte{0x06, 0x01, 0x04, 0x00, 0x61, 0x15, 0x50, 0x60}},
		{"InitFast", InitFast, []byte{0x00, 0x50, 0x04, 0x06, 0xE0, 0xE5}},
		{"InitPartial", InitPartial, []byte{0x00, 0x04, 0xE0, 0xE5}},
		{"Init4Gray", Init4Gray, []byte{0x00, 0x50, 0x04, 0x06, 0xE0, 0xE5}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &MockHardware{}
			epd := NewEPD(m, &Waveshare7in5V2)

			if err := epd.Init(tt.mode); err != nil {
				t.Fatal(err)
			}
			if m.Calls[0].Type != "reset" {
				t.Errorf("first call = %q, want reset", m.Calls[0].Type)
			}
			if cmds := m.Commands(); !bytes.Equal(cmds, tt.wantCmds) {
				t.Errorf("commands = %#v, want %#v", cmds, tt.wantCmds)
			}
		})
	}
}

func TestInitUnsupportedModeReturnsError(t *testing.T) {
	p := &DisplayProfile{
		Name:     "limited",
		Width:    8,
		Height:   8,
		Color:    BW,
		InitFull: []Command{{0x00, []byte{0x1F}}},
	}
	err := NewEPD(&MockHardware{}, p).Init(InitFast)
	if err == nil {
		t.Fatal("expected error for unsupported mode")
	}
}

func TestInitResetError(t *testing.T) {
	err := NewEPD(&errorResetHardware{}, &Waveshare7in5V2).Init(InitFull)
	if err == nil {
		t.Fatal("expected error from Reset")
	}
}

// --- Display ---

func TestDisplayCommandSequence(t *testing.T) {
	m := &MockHardware{}
	p := smallTestProfile()
	epd := NewEPD(m, p)

	buf := bytes.Repeat([]byte{0xAA}, p.BufferSize())
	if err := epd.Display(buf); err != nil {
		t.Fatal(err)
	}

	wantCmds := []byte{0x10, 0x13, 0x12}
	if cmds := m.Commands(); !bytes.Equal(cmds, wantCmds) {
		t.Errorf("commands = %#v, want %#v", cmds, wantCmds)
	}
}

func TestDisplayBufferInversion(t *testing.T) {
	m := &MockHardware{}
	p := smallTestProfile()
	epd := NewEPD(m, p)

	buf := bytes.Repeat([]byte{0xAA}, p.BufferSize())
	if err := epd.Display(buf); err != nil {
		t.Fatal(err)
	}

	dataCalls := m.DataCalls()
	if len(dataCalls) < 2 {
		t.Fatalf("expected at least 2 data calls, got %d", len(dataCalls))
	}

	wantOld := bytes.Repeat([]byte{0x55}, p.BufferSize())
	if !bytes.Equal(dataCalls[0], wantOld) {
		t.Errorf("old buffer[0] = %#x, want 0x55 (inverted)", dataCalls[0][0])
	}

	wantNew := bytes.Repeat([]byte{0xAA}, p.BufferSize())
	if !bytes.Equal(dataCalls[1], wantNew) {
		t.Errorf("new buffer[0] = %#x, want 0xAA (original)", dataCalls[1][0])
	}
}

func TestDisplayBufferSizeValidation(t *testing.T) {
	epd := NewEPD(&MockHardware{}, smallTestProfile())
	if err := epd.Display([]byte{0x00, 0x01}); err == nil {
		t.Fatal("expected error for wrong buffer size")
	}
}

func TestDisplaySendCommandErrors(t *testing.T) {
	p := smallTestProfile()
	buf := make([]byte, p.BufferSize())

	for _, n := range []int{1, 2, 3} {
		eh := &errorHardware{failOnCall: n}
		if err := NewEPD(eh, p).Display(buf); err == nil {
			t.Errorf("expected error on SendCommand #%d", n)
		}
	}
}

func TestDisplaySendDataErrors(t *testing.T) {
	p := smallTestProfile()
	buf := make([]byte, p.BufferSize())

	// Error on first SendData (old buffer)
	if err := NewEPD(&errorDataHardware{}, p).Display(buf); err == nil {
		t.Error("expected error on old buffer SendData")
	}

	// Error on second SendData (new buffer)
	if err := NewEPD(&errorDataNthHardware{failOnCall: 2}, p).Display(buf); err == nil {
		t.Error("expected error on new buffer SendData")
	}
}

// --- Display (Gray4 path) ---

func TestDisplayGray4CommandSequence(t *testing.T) {
	m := &MockHardware{}
	p := gray4DisplayProfile()
	epd := NewEPD(m, p)

	buf := make([]byte, p.BufferSize())
	if err := epd.Display(buf); err != nil {
		t.Fatal(err)
	}

	wantCmds := []byte{0x10, 0x13, 0x12}
	if cmds := m.Commands(); !bytes.Equal(cmds, wantCmds) {
		t.Errorf("commands = %#v, want %#v", cmds, wantCmds)
	}
}

// TestDisplayGray4PlaneSplit covers the full Gray4 wire protocol end-to-end
// with all four shades present. The buffer mirrors the upstream Waveshare
// driver's encoding (after Inkwell's white=00, black=11 mapping) and the
// expected plane bytes derive from the bit math: planeA = low bit per
// pixel, planeB = high bit. A regression that swaps planes or shifts
// shades will produce a different byte at a specific index, not a vague
// "looks wrong on hardware".
func TestDisplayGray4PlaneSplit(t *testing.T) {
	m := &MockHardware{}
	p := gray4DisplayProfile() // 16x2 = 32 px, 8 bytes 2bpp
	epd := NewEPD(m, p)

	// 32 pixels packed 2bpp into 8 bytes. The plane-split processes
	// these in pairs (each pair → 1 byte per plane). Each pair below
	// is constructed to exercise a known shade combination.
	buf := []byte{
		0x1B, 0xE4, // pair 0: mixed — w,l,d,b, b,d,l,w
		0x00, 0xFF, // pair 1: 4 white pixels then 4 black
		0x55, 0x55, // pair 2: 8 light-gray pixels
		0xAA, 0xAA, // pair 3: 8 dark-gray pixels
	}

	if err := epd.Display(buf); err != nil {
		t.Fatal(err)
	}

	dataCalls := m.DataCalls()
	if len(dataCalls) != 2 {
		t.Fatalf("expected 2 data calls (plane A + plane B), got %d", len(dataCalls))
	}

	wantA := []byte{0x5A, 0x0F, 0xFF, 0x00}
	wantB := []byte{0x3C, 0x0F, 0x00, 0xFF}
	if !bytes.Equal(dataCalls[0], wantA) {
		t.Errorf("plane A (0x10 payload) = % X, want % X", dataCalls[0], wantA)
	}
	if !bytes.Equal(dataCalls[1], wantB) {
		t.Errorf("plane B (0x13 payload) = % X, want % X", dataCalls[1], wantB)
	}
}

func TestDisplayGray4BufferSizeValidation(t *testing.T) {
	epd := NewEPD(&MockHardware{}, gray4DisplayProfile())
	if err := epd.Display([]byte{0x00, 0x01}); err == nil {
		t.Fatal("expected error for wrong buffer size")
	}
}

func TestDisplayGray4SendCommandErrors(t *testing.T) {
	p := gray4DisplayProfile()
	buf := make([]byte, p.BufferSize())

	// 3 SendCommand calls: OldBufferCmd, NewBufferCmd, RefreshCmd
	for _, n := range []int{1, 2, 3} {
		eh := &errorHardware{failOnCall: n}
		if err := NewEPD(eh, p).Display(buf); err == nil {
			t.Errorf("expected error on SendCommand #%d", n)
		}
	}
}

func TestDisplayGray4SendDataErrors(t *testing.T) {
	p := gray4DisplayProfile()
	buf := make([]byte, p.BufferSize())

	// 2 SendData calls: plane A, plane B
	if err := NewEPD(&errorDataHardware{}, p).Display(buf); err == nil {
		t.Error("expected error on plane A SendData")
	}
	if err := NewEPD(&errorDataNthHardware{failOnCall: 2}, p).Display(buf); err == nil {
		t.Error("expected error on plane B SendData")
	}
}

// TestDisplayUnsupportedColorDepth locks the behaviour for color depths the
// transport doesn't know how to drive. Color7 is declared in the ColorDepth
// enum for future expansion but has no wire protocol implementation in
// EPD.Display — calling Display with a Color7 profile must fail loudly
// rather than silently sending BW-shaped data.
func TestDisplayUnsupportedColorDepth(t *testing.T) {
	p := &DisplayProfile{
		Name:         "test-color7",
		Width:        16,
		Height:       2,
		Color:        Color7,
		OldBufferCmd: 0x10,
		NewBufferCmd: 0x13,
		RefreshCmd:   0x12,
	}
	buf := make([]byte, p.BufferSize())
	if err := NewEPD(&MockHardware{}, p).Display(buf); err == nil {
		t.Fatal("expected error for unsupported color depth, got nil")
	}
}

// --- Clear ---

// TestClearSendsWhiteBuffers locks the on-wire bytes Clear emits across
// both supported color depths. In Inkwell's encoding the "all white"
// sentinel byte is 0x00 for BW (packBW sets bit 1 for black, so 0=white)
// and 0x00 for Gray4 (white=00 → both plane bits clear). The upstream
// Waveshare reference confirms this: Clear() there writes 0xFF to 0x10
// and 0x00 to 0x13 — the new-frame plane is all-white, the previous-
// frame plane is its inversion. A regression that fills with 0xFF
// renders solid black, not clear, on real hardware.
func TestClearSendsWhiteBuffers(t *testing.T) {
	cases := []struct {
		label   string
		profile *DisplayProfile
		// What Display sees on the wire after Clear.
		wantOld []byte
		wantNew []byte
	}{
		{
			label:   "BW: 0x10←0xFF, 0x13←0x00",
			profile: smallTestProfile(),
			// 16x16 BW = 32 bytes.
			wantOld: bytes.Repeat([]byte{0xFF}, 32),
			wantNew: bytes.Repeat([]byte{0x00}, 32),
		},
		{
			label:   "Gray4: both planes all-zero (white pixels)",
			profile: gray4DisplayProfile(),
			// 16x2 Gray4 = 8 bytes 2bpp → 4 bytes per plane.
			wantOld: bytes.Repeat([]byte{0x00}, 4),
			wantNew: bytes.Repeat([]byte{0x00}, 4),
		},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			m := &MockHardware{}
			epd := NewEPD(m, tc.profile)
			if err := epd.Clear(); err != nil {
				t.Fatal(err)
			}

			wantCmds := []byte{0x10, 0x13, 0x12}
			if cmds := m.Commands(); !bytes.Equal(cmds, wantCmds) {
				t.Errorf("commands = %#v, want %#v", cmds, wantCmds)
			}

			dataCalls := m.DataCalls()
			if len(dataCalls) < 2 {
				t.Fatalf("expected at least 2 data calls, got %d", len(dataCalls))
			}
			if !bytes.Equal(dataCalls[0], tc.wantOld) {
				t.Errorf("old plane = % X, want % X", dataCalls[0], tc.wantOld)
			}
			if !bytes.Equal(dataCalls[1], tc.wantNew) {
				t.Errorf("new plane = % X, want % X", dataCalls[1], tc.wantNew)
			}
		})
	}
}

// --- Sleep / Close ---

func TestSleepSendsProfileSleepSequence(t *testing.T) {
	m := &MockHardware{}
	epd := NewEPD(m, &Waveshare7in5V2)

	if err := epd.Sleep(); err != nil {
		t.Fatal(err)
	}

	wantCmds := []byte{0x50, 0x02, 0x07}
	if cmds := m.Commands(); !bytes.Equal(cmds, wantCmds) {
		t.Errorf("commands = %#v, want %#v", cmds, wantCmds)
	}
}

func TestCloseSleepsThenCloses(t *testing.T) {
	m := &MockHardware{}
	epd := NewEPD(m, &Waveshare7in5V2)

	if err := epd.Close(); err != nil {
		t.Fatal(err)
	}

	last := m.Calls[len(m.Calls)-1]
	if last.Type != "close" {
		t.Errorf("last call = %q, want close", last.Type)
	}

	wantCmds := []byte{0x50, 0x02, 0x07}
	if cmds := m.Commands(); !bytes.Equal(cmds, wantCmds) {
		t.Errorf("commands = %#v, want %#v", cmds, wantCmds)
	}
}

func TestClosePropagatesSleepError(t *testing.T) {
	err := NewEPD(&errorHardware{failOnCall: 1}, &Waveshare7in5V2).Close()
	if err == nil {
		t.Fatal("expected error from Sleep during Close")
	}
}

// --- DisplayPartial ---

func TestDisplayPartialCommandSequence(t *testing.T) {
	m := &MockHardware{}
	epd := NewEPD(m, partialTestProfile())

	buf := make([]byte, 32) // 16x16 region: 16/8 * 16
	if err := epd.DisplayPartial(buf, Region{X: 0, Y: 0, W: 16, H: 16}); err != nil {
		t.Fatal(err)
	}

	wantCmds := []byte{0x50, 0x91, 0x90, 0x13, 0x12}
	if cmds := m.Commands(); !bytes.Equal(cmds, wantCmds) {
		t.Errorf("commands = %#v, want %#v", cmds, wantCmds)
	}
}

func TestDisplayPartialByteAlignment(t *testing.T) {
	m := &MockHardware{}
	epd := NewEPD(m, partialTestProfile())

	// x=13 aligns down to 8, w=20 + offset(5) = 25 aligns up to 32
	buf := make([]byte, 4*16) // (32/8)*16 = 64
	if err := epd.DisplayPartial(buf, Region{X: 13, Y: 0, W: 20, H: 16}); err != nil {
		t.Fatal(err)
	}

	wd := windowDataFromCalls(m.Calls)
	if wd == nil {
		t.Fatal("partial window data not found")
	}

	// Xstart = 8 (aligned from 13)
	if wd[0] != 0x00 || wd[1] != 0x08 {
		t.Errorf("Xstart = [%#x, %#x], want [0x00, 0x08]", wd[0], wd[1])
	}
	// Xend = 8 + 32 - 1 = 39
	if wd[2] != 0x00 || wd[3] != 0x27 {
		t.Errorf("Xend = [%#x, %#x], want [0x00, 0x27]", wd[2], wd[3])
	}
}

func TestDisplayPartialWindowEncoding(t *testing.T) {
	m := &MockHardware{}
	epd := NewEPD(m, partialTestProfile())

	// Region at x=256, y=100, w=32, h=50
	buf := make([]byte, (32/8)*50) // 200 bytes
	if err := epd.DisplayPartial(buf, Region{X: 256, Y: 100, W: 32, H: 50}); err != nil {
		t.Fatal(err)
	}

	wd := windowDataFromCalls(m.Calls)
	if len(wd) != 9 {
		t.Fatalf("window data length = %d, want 9", len(wd))
	}

	want := []byte{
		0x01, 0x00, // Xstart=256
		0x01, 0x1F, // Xend=287
		0x00, 0x64, // Ystart=100
		0x00, 0x95, // Yend=149
		0x01,       // Scan direction
	}
	if !bytes.Equal(wd, want) {
		t.Errorf("window data = %#v, want %#v", wd, want)
	}
}

func TestDisplayPartialUnsupportedProfile(t *testing.T) {
	p := &DisplayProfile{
		Name:         "no-partial",
		Width:        16,
		Height:       16,
		Color:        BW,
		Capabilities: Capabilities{PartialRefresh: false},
	}
	err := NewEPD(&MockHardware{}, p).DisplayPartial([]byte{0x00}, Region{X: 0, Y: 0, W: 8, H: 1})
	if err == nil {
		t.Fatal("expected error for unsupported partial refresh")
	}
}

func TestDisplayPartialSendCommandErrors(t *testing.T) {
	p := partialTestProfile()
	buf := make([]byte, 32)
	region := Region{X: 0, Y: 0, W: 16, H: 16}

	// 5 SendCommand calls: VCOM, enter, window, data, refresh
	for _, n := range []int{1, 2, 3, 4, 5} {
		eh := &errorHardware{failOnCall: n}
		if err := NewEPD(eh, p).DisplayPartial(buf, region); err == nil {
			t.Errorf("expected error on SendCommand #%d", n)
		}
	}
}

func TestDisplayPartialSendDataErrors(t *testing.T) {
	p := partialTestProfile()
	buf := make([]byte, 32)
	region := Region{X: 0, Y: 0, W: 16, H: 16}

	// 3 SendData calls: VCOM data, window data, buffer data
	for _, n := range []int{1, 2, 3} {
		ed := &errorDataNthHardware{failOnCall: n}
		if err := NewEPD(ed, p).DisplayPartial(buf, region); err == nil {
			t.Errorf("expected error on SendData #%d", n)
		}
	}
}
