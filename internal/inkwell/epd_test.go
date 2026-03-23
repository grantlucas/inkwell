package inkwell

import (
	"bytes"
	"errors"
	"testing"
)

func TestNewEPD(t *testing.T) {
	m := &MockHardware{}
	p := &Waveshare7in5V2
	epd := NewEPD(m, p)
	if epd.Width() != 800 {
		t.Errorf("Width() = %d, want 800", epd.Width())
	}
	if epd.Height() != 480 {
		t.Errorf("Height() = %d, want 480", epd.Height())
	}
}

func TestExecSequenceThreeCommands(t *testing.T) {
	m := &MockHardware{}
	p := &DisplayProfile{Name: "test", Width: 8, Height: 8, Color: BW}
	epd := NewEPD(m, p)

	seq := []Command{
		{0x06, []byte{0x17, 0x17}},  // command with data
		{0x04, nil},                  // command without data (triggers busy wait)
		{0x00, []byte{0x1F}},        // command with data
	}

	if err := epd.execSequence(seq); err != nil {
		t.Fatal(err)
	}

	// Expected call sequence:
	// command(0x06), data(0x17,0x17), command(0x04), busy, command(0x00), data(0x1F)
	if len(m.Calls) != 6 {
		t.Fatalf("len(Calls) = %d, want 6; calls: %v", len(m.Calls), m.Calls)
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

func TestExecSequenceErrorPropagation(t *testing.T) {
	eh := &errorHardware{failOnCall: 2} // fail on second SendCommand
	p := &DisplayProfile{Name: "test", Width: 8, Height: 8, Color: BW}
	epd := NewEPD(eh, p)

	seq := []Command{
		{0x06, []byte{0x17}},
		{0x04, nil},         // this SendCommand should fail
		{0x00, []byte{0x1F}},
	}

	err := epd.execSequence(seq)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "SPI write failed" {
		t.Errorf("error = %q, want %q", err.Error(), "SPI write failed")
	}
}

// errorDataHardware returns an error on SendData.
type errorDataHardware struct {
	MockHardware
}

func (e *errorDataHardware) SendData(data []byte) error {
	return errors.New("SPI data write failed")
}

func TestExecSequenceSendDataError(t *testing.T) {
	eh := &errorDataHardware{}
	p := &DisplayProfile{Name: "test", Width: 8, Height: 8, Color: BW}
	epd := NewEPD(eh, p)

	seq := []Command{
		{0x06, []byte{0x17}}, // SendData will fail
	}

	err := epd.execSequence(seq)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestInitFullSendsResetThenProfileCommands(t *testing.T) {
	m := &MockHardware{}
	epd := NewEPD(m, &Waveshare7in5V2)

	if err := epd.Init(InitFull); err != nil {
		t.Fatal(err)
	}

	// First call should be reset
	if m.Calls[0].Type != "reset" {
		t.Errorf("first call = %q, want reset", m.Calls[0].Type)
	}

	// Command bytes after reset should match InitFull sequence
	cmds := m.Commands()
	wantCmds := []byte{0x06, 0x01, 0x04, 0x00, 0x61, 0x15, 0x50, 0x60}
	if !bytes.Equal(cmds, wantCmds) {
		t.Errorf("commands = %#v, want %#v", cmds, wantCmds)
	}
}

func TestInitFastSendsCorrectSequence(t *testing.T) {
	m := &MockHardware{}
	epd := NewEPD(m, &Waveshare7in5V2)

	if err := epd.Init(InitFast); err != nil {
		t.Fatal(err)
	}

	if m.Calls[0].Type != "reset" {
		t.Errorf("first call = %q, want reset", m.Calls[0].Type)
	}

	cmds := m.Commands()
	wantCmds := []byte{0x00, 0x50, 0x04, 0x06, 0xE0, 0xE5}
	if !bytes.Equal(cmds, wantCmds) {
		t.Errorf("commands = %#v, want %#v", cmds, wantCmds)
	}
}

func TestInitPartialSendsCorrectSequence(t *testing.T) {
	m := &MockHardware{}
	epd := NewEPD(m, &Waveshare7in5V2)

	if err := epd.Init(InitPartial); err != nil {
		t.Fatal(err)
	}

	if m.Calls[0].Type != "reset" {
		t.Errorf("first call = %q, want reset", m.Calls[0].Type)
	}

	cmds := m.Commands()
	wantCmds := []byte{0x00, 0x04, 0xE0, 0xE5}
	if !bytes.Equal(cmds, wantCmds) {
		t.Errorf("commands = %#v, want %#v", cmds, wantCmds)
	}
}

func TestInit4GraySendsCorrectSequence(t *testing.T) {
	m := &MockHardware{}
	epd := NewEPD(m, &Waveshare7in5V2)

	if err := epd.Init(Init4Gray); err != nil {
		t.Fatal(err)
	}

	if m.Calls[0].Type != "reset" {
		t.Errorf("first call = %q, want reset", m.Calls[0].Type)
	}

	cmds := m.Commands()
	wantCmds := []byte{0x00, 0x50, 0x04, 0x06, 0xE0, 0xE5}
	if !bytes.Equal(cmds, wantCmds) {
		t.Errorf("commands = %#v, want %#v", cmds, wantCmds)
	}
}

func TestInitUnsupportedModeReturnsError(t *testing.T) {
	m := &MockHardware{}
	// Profile with no InitFast sequence
	p := &DisplayProfile{
		Name:     "limited",
		Width:    8,
		Height:   8,
		Color:    BW,
		InitFull: []Command{{0x00, []byte{0x1F}}},
	}
	epd := NewEPD(m, p)

	err := epd.Init(InitFast)
	if err == nil {
		t.Fatal("expected error for unsupported mode")
	}
}

func TestInitResetError(t *testing.T) {
	eh := &errorResetHardware{}
	epd := NewEPD(eh, &Waveshare7in5V2)

	err := epd.Init(InitFull)
	if err == nil {
		t.Fatal("expected error from Reset")
	}
}

type errorResetHardware struct {
	MockHardware
}

func (e *errorResetHardware) Reset() error {
	return errors.New("reset failed")
}

func TestDisplayCommandSequence(t *testing.T) {
	m := &MockHardware{}
	p := &DisplayProfile{
		Name:         "test",
		Width:        16,
		Height:       16,
		Color:        BW,
		OldBufferCmd: 0x10,
		NewBufferCmd: 0x13,
		RefreshCmd:   0x12,
	}
	epd := NewEPD(m, p)

	bufSize := p.BufferSize() // 16*16/8 = 32
	buf := make([]byte, bufSize)
	for i := range buf {
		buf[i] = 0xAA
	}

	if err := epd.Display(buf); err != nil {
		t.Fatal(err)
	}

	// Expected: command(0x10), data(inverted), command(0x13), data(original), command(0x12), busy
	cmds := m.Commands()
	wantCmds := []byte{0x10, 0x13, 0x12}
	if !bytes.Equal(cmds, wantCmds) {
		t.Errorf("commands = %#v, want %#v", cmds, wantCmds)
	}
}

func TestDisplayBufferInversion(t *testing.T) {
	m := &MockHardware{}
	p := &DisplayProfile{
		Name:         "test",
		Width:        16,
		Height:       16,
		Color:        BW,
		OldBufferCmd: 0x10,
		NewBufferCmd: 0x13,
		RefreshCmd:   0x12,
	}
	epd := NewEPD(m, p)

	buf := make([]byte, p.BufferSize())
	for i := range buf {
		buf[i] = 0xAA // 10101010
	}

	if err := epd.Display(buf); err != nil {
		t.Fatal(err)
	}

	// Find the first data call (old buffer, should be inverted)
	var oldData, newData []byte
	dataIdx := 0
	for _, c := range m.Calls {
		if c.Type == "data" {
			if dataIdx == 0 {
				oldData = c.Data
			} else {
				newData = c.Data
			}
			dataIdx++
		}
	}

	// Old buffer should be inverted: ^0xAA = 0x55
	for i, b := range oldData {
		if b != 0x55 {
			t.Errorf("old buffer[%d] = %#x, want 0x55", i, b)
			break
		}
	}

	// New buffer should be original
	for i, b := range newData {
		if b != 0xAA {
			t.Errorf("new buffer[%d] = %#x, want 0xAA", i, b)
			break
		}
	}
}

func TestDisplaySendCommandErrors(t *testing.T) {
	p := &DisplayProfile{
		Name:         "test",
		Width:        16,
		Height:       16,
		Color:        BW,
		OldBufferCmd: 0x10,
		NewBufferCmd: 0x13,
		RefreshCmd:   0x12,
	}
	buf := make([]byte, p.BufferSize())

	// Error on 1st SendCommand (OldBufferCmd)
	eh1 := &errorHardware{failOnCall: 1}
	epd1 := NewEPD(eh1, p)
	if err := epd1.Display(buf); err == nil {
		t.Error("expected error on OldBufferCmd SendCommand")
	}

	// Error on 2nd SendCommand (NewBufferCmd)
	eh2 := &errorHardware{failOnCall: 2}
	epd2 := NewEPD(eh2, p)
	if err := epd2.Display(buf); err == nil {
		t.Error("expected error on NewBufferCmd SendCommand")
	}

	// Error on 3rd SendCommand (RefreshCmd)
	eh3 := &errorHardware{failOnCall: 3}
	epd3 := NewEPD(eh3, p)
	if err := epd3.Display(buf); err == nil {
		t.Error("expected error on RefreshCmd SendCommand")
	}
}

func TestDisplaySendDataErrors(t *testing.T) {
	p := &DisplayProfile{
		Name:         "test",
		Width:        16,
		Height:       16,
		Color:        BW,
		OldBufferCmd: 0x10,
		NewBufferCmd: 0x13,
		RefreshCmd:   0x12,
	}
	buf := make([]byte, p.BufferSize())

	// Error on first SendData (old buffer)
	eh := &errorDataHardware{}
	epd := NewEPD(eh, p)
	if err := epd.Display(buf); err == nil {
		t.Error("expected error on old buffer SendData")
	}

	// Error on second SendData (new buffer) — need a mock that fails on 2nd call
	eh2 := &errorDataNthHardware{failOnCall: 2}
	epd2 := NewEPD(eh2, p)
	if err := epd2.Display(buf); err == nil {
		t.Error("expected error on new buffer SendData")
	}
}

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

func TestClearSendsWhiteBuffers(t *testing.T) {
	m := &MockHardware{}
	p := &DisplayProfile{
		Name:         "test",
		Width:        16,
		Height:       16,
		Color:        BW,
		OldBufferCmd: 0x10,
		NewBufferCmd: 0x13,
		RefreshCmd:   0x12,
	}
	epd := NewEPD(m, p)

	if err := epd.Clear(); err != nil {
		t.Fatal(err)
	}

	cmds := m.Commands()
	wantCmds := []byte{0x10, 0x13, 0x12}
	if !bytes.Equal(cmds, wantCmds) {
		t.Errorf("commands = %#v, want %#v", cmds, wantCmds)
	}

	// Old buffer should be 0x00 (inverted 0xFF = 0x00)
	// New buffer should be 0xFF (white)
	dataIdx := 0
	for _, c := range m.Calls {
		if c.Type == "data" {
			if dataIdx == 0 {
				// Old buffer: Clear passes 0xFF, which gets inverted to 0x00
				for i, b := range c.Data {
					if b != 0x00 {
						t.Errorf("old buffer[%d] = %#x, want 0x00", i, b)
						break
					}
				}
			} else {
				// New buffer: 0xFF (white)
				for i, b := range c.Data {
					if b != 0xFF {
						t.Errorf("new buffer[%d] = %#x, want 0xFF", i, b)
						break
					}
				}
			}
			dataIdx++
		}
	}
}

func TestSleepSendsProfileSleepSequence(t *testing.T) {
	m := &MockHardware{}
	epd := NewEPD(m, &Waveshare7in5V2)

	if err := epd.Sleep(); err != nil {
		t.Fatal(err)
	}

	cmds := m.Commands()
	// Sleep sequence: 0x50 (VCOM), 0x02 (power off), 0x07 (deep sleep)
	wantCmds := []byte{0x50, 0x02, 0x07}
	if !bytes.Equal(cmds, wantCmds) {
		t.Errorf("commands = %#v, want %#v", cmds, wantCmds)
	}
}

func TestCloseSleepsThenCloses(t *testing.T) {
	m := &MockHardware{}
	epd := NewEPD(m, &Waveshare7in5V2)

	if err := epd.Close(); err != nil {
		t.Fatal(err)
	}

	// Last call should be close
	last := m.Calls[len(m.Calls)-1]
	if last.Type != "close" {
		t.Errorf("last call = %q, want close", last.Type)
	}

	// Should have sleep commands before close
	cmds := m.Commands()
	wantCmds := []byte{0x50, 0x02, 0x07}
	if !bytes.Equal(cmds, wantCmds) {
		t.Errorf("commands = %#v, want %#v", cmds, wantCmds)
	}
}

func TestClosePropagatesSleepError(t *testing.T) {
	// Use errorHardware that fails on first SendCommand (Sleep's first cmd)
	eh := &errorHardware{failOnCall: 1}
	epd := NewEPD(eh, &Waveshare7in5V2)

	err := epd.Close()
	if err == nil {
		t.Fatal("expected error from Sleep during Close")
	}
}

func TestDisplayBufferSizeValidation(t *testing.T) {
	m := &MockHardware{}
	p := &DisplayProfile{
		Name:         "test",
		Width:        16,
		Height:       16,
		Color:        BW,
		OldBufferCmd: 0x10,
		NewBufferCmd: 0x13,
		RefreshCmd:   0x12,
	}
	epd := NewEPD(m, p)

	// Wrong size buffer
	err := epd.Display([]byte{0x00, 0x01})
	if err == nil {
		t.Fatal("expected error for wrong buffer size")
	}
}
