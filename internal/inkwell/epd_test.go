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
