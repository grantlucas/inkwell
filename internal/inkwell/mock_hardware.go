package inkwell

// Call records a single operation on MockHardware.
type Call struct {
	Type string // "command", "data", "busy", "reset", "close"
	Data []byte
}

// MockHardware is a test double that records every SPI/GPIO call
// for assertion in tests. It implements the Hardware interface.
type MockHardware struct {
	Calls     []Call
	BusyCount int // Number of ReadBusy calls that return false (busy) before returning true (idle)
	busyRead  int
}

// SendCommand records a command byte.
func (m *MockHardware) SendCommand(cmd byte) error {
	m.Calls = append(m.Calls, Call{Type: "command", Data: []byte{cmd}})
	return nil
}

// SendData records a data payload.
func (m *MockHardware) SendData(data []byte) error {
	cp := make([]byte, len(data))
	copy(cp, data)
	m.Calls = append(m.Calls, Call{Type: "data", Data: cp})
	return nil
}

// ReadBusy simulates the BUSY pin. Returns false (busy) for BusyCount reads,
// then returns true (idle). Default BusyCount of 0 means always idle.
func (m *MockHardware) ReadBusy() bool {
	m.Calls = append(m.Calls, Call{Type: "busy"})
	if m.busyRead < m.BusyCount {
		m.busyRead++
		return false
	}
	return true
}

// Reset records a reset call.
func (m *MockHardware) Reset() error {
	m.Calls = append(m.Calls, Call{Type: "reset"})
	return nil
}

// Close records a close call.
func (m *MockHardware) Close() error {
	m.Calls = append(m.Calls, Call{Type: "close"})
	return nil
}

// Commands returns just the command register bytes, filtering out data/busy/reset calls.
// Useful for quick assertions on the command sequence without checking payloads.
func (m *MockHardware) Commands() []byte {
	var cmds []byte
	for _, c := range m.Calls {
		if c.Type == "command" {
			cmds = append(cmds, c.Data[0])
		}
	}
	return cmds
}
