//go:build hardware

package inkwell

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpiotest"
	"periph.io/x/conn/v3/physic"
	"periph.io/x/conn/v3/spi"
	"periph.io/x/conn/v3/spi/spitest"
)

func newTestSPIHardware(t *testing.T) (*spiHardware, *spitest.Record, *gpiotest.Pin, *gpiotest.Pin, *gpiotest.Pin, *gpiotest.Pin) {
	t.Helper()

	record := &spitest.Record{}
	conn, err := record.Connect(0, 0, 8)
	if err != nil {
		t.Fatalf("spitest.Record.Connect: %v", err)
	}

	rstPin := &gpiotest.Pin{N: "RST", Num: 17}
	dcPin := &gpiotest.Pin{N: "DC", Num: 25}
	busyPin := &gpiotest.Pin{N: "BUSY", Num: 24}
	pwrPin := &gpiotest.Pin{N: "PWR", Num: 18}

	hw, err := NewSPIHardware(
		WithSPIConn(conn),
		WithGPIOPins(rstPin, dcPin, busyPin, pwrPin),
	)
	if err != nil {
		t.Fatalf("NewSPIHardware: %v", err)
	}

	return hw, record, rstPin, dcPin, busyPin, pwrPin
}

func TestCreateBackend_SPI_WithHardwareTag(t *testing.T) {
	// With the hardware tag, createBackend("spi") calls NewSPIHardware() which
	// tries initRealHardware and fails in a non-Pi environment.
	cfg := &Config{Backend: "spi"}
	profile := &Waveshare7in5V2
	_, err := createBackend(cfg, profile)
	if err == nil {
		t.Fatal("expected error from createBackend on non-Pi")
	}
}

func TestNewApp_SPI_WithHardwareTag(t *testing.T) {
	cfg, err := LoadConfig(strings.NewReader(`
display: waveshare_7in5_v2
backend: spi
`))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	_, err = NewApp(cfg)
	if err == nil {
		t.Fatal("expected error from NewApp on non-Pi")
	}
}

func TestSPIHardware_ImplementsHardware(t *testing.T) {
	var _ Hardware = (*spiHardware)(nil)
}

func TestSPIHardware_SendCommand_SetsDCLow(t *testing.T) {
	hw, record, _, dcPin, _, _ := newTestSPIHardware(t)

	if err := hw.SendCommand(0x04); err != nil {
		t.Fatalf("SendCommand: %v", err)
	}

	if dcPin.L != gpio.Low {
		t.Errorf("DC pin = %v, want Low", dcPin.L)
	}
	if len(record.Ops) != 1 {
		t.Fatalf("SPI ops = %d, want 1", len(record.Ops))
	}
	if !bytes.Equal(record.Ops[0].W, []byte{0x04}) {
		t.Errorf("SPI write = %#v, want [0x04]", record.Ops[0].W)
	}
}

func TestSPIHardware_SendData_SetsDCHigh(t *testing.T) {
	hw, record, _, dcPin, _, _ := newTestSPIHardware(t)

	data := []byte{0x17, 0x17, 0x65, 0x01}
	if err := hw.SendData(data); err != nil {
		t.Fatalf("SendData: %v", err)
	}

	if dcPin.L != gpio.High {
		t.Errorf("DC pin = %v, want High", dcPin.L)
	}
	if len(record.Ops) != 1 {
		t.Fatalf("SPI ops = %d, want 1", len(record.Ops))
	}
	if !bytes.Equal(record.Ops[0].W, data) {
		t.Errorf("SPI write = %#v, want %#v", record.Ops[0].W, data)
	}
}

func TestSPIHardware_ReadBusy_HighMeansIdle(t *testing.T) {
	hw, _, _, _, busyPin, _ := newTestSPIHardware(t)

	busyPin.L = gpio.High
	if !hw.ReadBusy() {
		t.Error("ReadBusy() = false, want true (idle) when BUSY=HIGH")
	}
}

func TestSPIHardware_ReadBusy_LowMeansBusy(t *testing.T) {
	hw, _, _, _, busyPin, _ := newTestSPIHardware(t)

	busyPin.L = gpio.Low
	if hw.ReadBusy() {
		t.Error("ReadBusy() = true, want false (busy) when BUSY=LOW")
	}
}

func TestSPIHardware_Reset_PulseSequence(t *testing.T) {
	hw, _, rstPin, _, _, _ := newTestSPIHardware(t)

	if err := hw.Reset(); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	// After reset, RST should be HIGH (final state).
	if rstPin.L != gpio.High {
		t.Errorf("RST pin after Reset = %v, want High", rstPin.L)
	}
}

func TestSPIHardware_Close_PowersDown(t *testing.T) {
	hw, _, _, _, _, pwrPin := newTestSPIHardware(t)

	if err := hw.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if pwrPin.L != gpio.Low {
		t.Errorf("PWR pin after Close = %v, want Low", pwrPin.L)
	}
}

func TestSPIHardware_Close_CollectsErrors(t *testing.T) {
	record := &spitest.Record{}
	conn, err := record.Connect(0, 0, 8)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}

	pwrErr := errors.New("pwr fail")
	failPWR := &failOutPin{Pin: gpiotest.Pin{N: "PWR", Num: 18}, err: pwrErr}

	hw, err := NewSPIHardware(
		WithSPIConn(conn),
		WithGPIOPins(
			&gpiotest.Pin{N: "RST", Num: 17},
			&gpiotest.Pin{N: "DC", Num: 25},
			&gpiotest.Pin{N: "BUSY", Num: 24},
			failPWR,
		),
	)
	if err != nil {
		t.Fatalf("NewSPIHardware: %v", err)
	}

	closeErr := hw.Close()
	if closeErr == nil {
		t.Fatal("expected error from Close")
	}
	if !errors.Is(closeErr, pwrErr) {
		t.Errorf("Close error = %v, want to contain %v", closeErr, pwrErr)
	}
}

// failOutPin wraps gpiotest.Pin but fails on Out().
type failOutPin struct {
	gpiotest.Pin
	err error
}

func (p *failOutPin) Out(l gpio.Level) error {
	return p.err
}

// failAfterNPin wraps gpiotest.Pin and fails on the Nth call to Out().
type failAfterNPin struct {
	gpiotest.Pin
	err   error
	n     int // fail on this call number (1-indexed)
	calls int
}

func (p *failAfterNPin) Out(l gpio.Level) error {
	p.calls++
	if p.calls == p.n {
		return p.err
	}
	p.Pin.L = l
	return nil
}

func TestSPIHardware_WithSPIConn(t *testing.T) {
	record := &spitest.Record{}
	conn, err := record.Connect(0, 0, 8)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}

	hw, err := NewSPIHardware(
		WithSPIConn(conn),
		WithGPIOPins(
			&gpiotest.Pin{N: "RST", Num: 17},
			&gpiotest.Pin{N: "DC", Num: 25},
			&gpiotest.Pin{N: "BUSY", Num: 24},
			&gpiotest.Pin{N: "PWR", Num: 18},
		),
	)
	if err != nil {
		t.Fatalf("NewSPIHardware: %v", err)
	}

	if err := hw.SendCommand(0x01); err != nil {
		t.Fatalf("SendCommand: %v", err)
	}
	if len(record.Ops) != 1 {
		t.Errorf("SPI ops = %d, want 1", len(record.Ops))
	}
}

func TestSPIHardware_WithGPIOPins(t *testing.T) {
	record := &spitest.Record{}
	conn, err := record.Connect(0, 0, 8)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}

	rstPin := &gpiotest.Pin{N: "RST", Num: 17}
	dcPin := &gpiotest.Pin{N: "DC", Num: 25}
	busyPin := &gpiotest.Pin{N: "BUSY", Num: 24}
	pwrPin := &gpiotest.Pin{N: "PWR", Num: 18}

	hw, err := NewSPIHardware(
		WithSPIConn(conn),
		WithGPIOPins(rstPin, dcPin, busyPin, pwrPin),
	)
	if err != nil {
		t.Fatalf("NewSPIHardware: %v", err)
	}

	busyPin.L = gpio.High
	if !hw.ReadBusy() {
		t.Error("BUSY pin not wired correctly")
	}
}

func TestNewSPIHardware_MissingSPIConn(t *testing.T) {
	// port is set (skipping initRealHardware) but conn is nil.
	_, err := NewSPIHardware(
		withPort(&spitest.Record{}),
		WithGPIOPins(
			&gpiotest.Pin{N: "RST"},
			&gpiotest.Pin{N: "DC"},
			&gpiotest.Pin{N: "BUSY"},
			&gpiotest.Pin{N: "PWR"},
		),
	)
	if err == nil {
		t.Fatal("expected error when SPI conn is nil")
	}
}

// withPort sets the port field directly so initRealHardware is skipped.
func withPort(p spi.PortCloser) SPIOption {
	return func(h *spiHardware) {
		h.port = p
	}
}

func TestNewSPIHardware_MissingGPIOPins(t *testing.T) {
	record := &spitest.Record{}
	conn, err := record.Connect(0, 0, 8)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}

	_, err = NewSPIHardware(WithSPIConn(conn))
	if err == nil {
		t.Fatal("expected error when GPIO pins are nil")
	}
}

func TestSPIHardware_DCPinError(t *testing.T) {
	dcErr := errors.New("dc fail")
	hw := newHardwareWithFailPin(t, "DC", dcErr)

	t.Run("SendCommand", func(t *testing.T) {
		if err := hw.SendCommand(0x01); !errors.Is(err, dcErr) {
			t.Errorf("error = %v, want %v", err, dcErr)
		}
	})

	t.Run("SendData", func(t *testing.T) {
		if err := hw.SendData([]byte{0x01}); !errors.Is(err, dcErr) {
			t.Errorf("error = %v, want %v", err, dcErr)
		}
	})
}

func TestSPIHardware_Reset_RstPinError(t *testing.T) {
	rstErr := errors.New("rst fail")
	hw := newHardwareWithFailPin(t, "RST", rstErr)

	if err := hw.Reset(); !errors.Is(err, rstErr) {
		t.Errorf("Reset error = %v, want %v", err, rstErr)
	}
}

// newHardwareWithFailPin creates an spiHardware with a failOutPin on the named pin.
func newHardwareWithFailPin(t *testing.T, pinName string, pinErr error) *spiHardware {
	t.Helper()
	record := &spitest.Record{}
	conn, err := record.Connect(0, 0, 8)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}

	rst := gpio.PinIO(&gpiotest.Pin{N: "RST"})
	dc := gpio.PinIO(&gpiotest.Pin{N: "DC"})
	busy := gpio.PinIO(&gpiotest.Pin{N: "BUSY"})
	pwr := gpio.PinIO(&gpiotest.Pin{N: "PWR"})

	switch pinName {
	case "RST":
		rst = &failOutPin{Pin: gpiotest.Pin{N: "RST"}, err: pinErr}
	case "DC":
		dc = &failOutPin{Pin: gpiotest.Pin{N: "DC"}, err: pinErr}
	case "PWR":
		pwr = &failOutPin{Pin: gpiotest.Pin{N: "PWR"}, err: pinErr}
	}

	hw, err := NewSPIHardware(WithSPIConn(conn), WithGPIOPins(rst, dc, busy, pwr))
	if err != nil {
		t.Fatalf("NewSPIHardware: %v", err)
	}
	return hw
}

// failPortCloser is an spi.PortCloser that records Close() calls and can fail.
type failPortCloser struct {
	spitest.Record
	closeErr error
}

func (f *failPortCloser) Close() error {
	return f.closeErr
}

func (f *failPortCloser) LimitSpeed(_ physic.Frequency) error {
	return nil
}

func TestSPIHardware_Reset_NthOutError(t *testing.T) {
	for _, tc := range []struct {
		name  string
		failN int
	}{
		{"second_out_low", 2},
		{"third_out_high", 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			record := &spitest.Record{}
			conn, err := record.Connect(0, 0, 8)
			if err != nil {
				t.Fatalf("Connect: %v", err)
			}

			rstErr := errors.New("rst fail")
			hw, err := NewSPIHardware(
				WithSPIConn(conn),
				WithGPIOPins(
					&failAfterNPin{Pin: gpiotest.Pin{N: "RST"}, err: rstErr, n: tc.failN},
					&gpiotest.Pin{N: "DC"},
					&gpiotest.Pin{N: "BUSY"},
					&gpiotest.Pin{N: "PWR"},
				),
			)
			if err != nil {
				t.Fatalf("NewSPIHardware: %v", err)
			}

			if err := hw.Reset(); !errors.Is(err, rstErr) {
				t.Errorf("Reset error = %v, want %v", err, rstErr)
			}
		})
	}
}

func TestNewSPIHardware_InitRealHardwareFallback(t *testing.T) {
	// When no WithSPIConn is provided and no port, initRealHardware returns error.
	_, err := NewSPIHardware()
	if err == nil {
		t.Fatal("expected error from initRealHardware")
	}
}

func TestSPIHardware_Close_PortCloseError(t *testing.T) {
	portErr := errors.New("port close fail")
	port := &failPortCloser{closeErr: portErr}
	conn, err := port.Connect(0, 0, 8)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}

	hw, err := NewSPIHardware(
		WithSPIConn(conn),
		WithGPIOPins(
			&gpiotest.Pin{N: "RST"},
			&gpiotest.Pin{N: "DC"},
			&gpiotest.Pin{N: "BUSY"},
			&gpiotest.Pin{N: "PWR"},
		),
	)
	if err != nil {
		t.Fatalf("NewSPIHardware: %v", err)
	}
	hw.port = port

	closeErr := hw.Close()
	if !errors.Is(closeErr, portErr) {
		t.Errorf("Close error = %v, want %v", closeErr, portErr)
	}
}
