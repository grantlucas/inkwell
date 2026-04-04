package inkwell

import (
	"bytes"
	"testing"
)

// Compile-time check that MockHardware implements Hardware.
var _ Hardware = (*MockHardware)(nil)

func TestMockHardwareRecordsSendCommand(t *testing.T) {
	m := &MockHardware{}
	if err := m.SendCommand(0x04); err != nil {
		t.Fatal(err)
	}
	if len(m.Calls) != 1 {
		t.Fatalf("len(Calls) = %d, want 1", len(m.Calls))
	}
	c := m.Calls[0]
	if c.Type != "command" {
		t.Errorf("Type = %q, want %q", c.Type, "command")
	}
	if !bytes.Equal(c.Data, []byte{0x04}) {
		t.Errorf("Data = %v, want [0x04]", c.Data)
	}
}

func TestMockHardwareRecordsSendData(t *testing.T) {
	m := &MockHardware{}
	data := []byte{0x17, 0x17, 0x28}
	if err := m.SendData(data); err != nil {
		t.Fatal(err)
	}
	if len(m.Calls) != 1 {
		t.Fatalf("len(Calls) = %d, want 1", len(m.Calls))
	}
	c := m.Calls[0]
	if c.Type != "data" {
		t.Errorf("Type = %q, want %q", c.Type, "data")
	}
	if !bytes.Equal(c.Data, data) {
		t.Errorf("Data = %v, want %v", c.Data, data)
	}
}

func TestMockHardwareRecordsReset(t *testing.T) {
	m := &MockHardware{}
	if err := m.Reset(); err != nil {
		t.Fatal(err)
	}
	if len(m.Calls) != 1 {
		t.Fatalf("len(Calls) = %d, want 1", len(m.Calls))
	}
	if m.Calls[0].Type != "reset" {
		t.Errorf("Type = %q, want %q", m.Calls[0].Type, "reset")
	}
}

func TestMockHardwareReadBusyDefault(t *testing.T) {
	m := &MockHardware{}
	if !m.ReadBusy() {
		t.Error("ReadBusy() = false, want true (idle by default)")
	}
	// Should record the busy call
	if len(m.Calls) != 1 {
		t.Fatalf("len(Calls) = %d, want 1", len(m.Calls))
	}
	if m.Calls[0].Type != "busy" {
		t.Errorf("Type = %q, want %q", m.Calls[0].Type, "busy")
	}
}

func TestMockHardwareBusyCount(t *testing.T) {
	m := &MockHardware{BusyCount: 3}

	// First 3 reads should return false (busy)
	for i := range 3 {
		if m.ReadBusy() {
			t.Errorf("ReadBusy() call %d = true, want false (busy)", i)
		}
	}
	// 4th read should return true (idle)
	if !m.ReadBusy() {
		t.Error("ReadBusy() after BusyCount = false, want true")
	}
}

func TestMockHardwareClose(t *testing.T) {
	m := &MockHardware{}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	if len(m.Calls) != 1 {
		t.Fatalf("len(Calls) = %d, want 1", len(m.Calls))
	}
	if m.Calls[0].Type != "close" {
		t.Errorf("Type = %q, want %q", m.Calls[0].Type, "close")
	}
}

func TestMockHardwareCommands(t *testing.T) {
	m := &MockHardware{}
	m.SendCommand(0x06)
	m.SendData([]byte{0x17, 0x17})
	m.SendCommand(0x04)
	m.ReadBusy()
	m.SendCommand(0x00)
	m.SendData([]byte{0x1F})

	got := m.Commands()
	want := []byte{0x06, 0x04, 0x00}
	if !bytes.Equal(got, want) {
		t.Errorf("Commands() = %v, want %v", got, want)
	}
}

func TestMockHardwareDataCalls(t *testing.T) {
	m := &MockHardware{}
	m.SendCommand(0x10)
	m.SendData([]byte{0xAA, 0xBB})
	m.SendCommand(0x13)
	m.SendData([]byte{0xCC})

	got := m.DataCalls()
	if len(got) != 2 {
		t.Fatalf("DataCalls() length = %d, want 2", len(got))
	}
	if !bytes.Equal(got[0], []byte{0xAA, 0xBB}) {
		t.Errorf("DataCalls()[0] = %v, want [0xAA, 0xBB]", got[0])
	}
	if !bytes.Equal(got[1], []byte{0xCC}) {
		t.Errorf("DataCalls()[1] = %v, want [0xCC]", got[1])
	}
}
