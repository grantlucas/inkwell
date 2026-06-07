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

// withInjections swaps the periph.io seam functions for the body of
// the test and restores them via t.Cleanup. A nil seam is replaced
// with a panic stub so a test that doesn't expect a particular seam
// to be hit will fail loudly if a refactor reorders initRealHardware
// — never silently falling through to the real periph.io call (which
// would touch /dev/spidev0.0 and the GPIO chip on a Pi).
//
// Not safe under t.Parallel(): the seam vars are package-level.
func withInjections(t *testing.T, hostFn func() error, openFn func(string) (spi.PortCloser, error), pinFn func(string) gpio.PinIO, body func()) {
	t.Helper()
	if hostFn == nil {
		hostFn = func() error { panic("hostInitFn called but test did not stub it") }
	}
	if openFn == nil {
		openFn = func(string) (spi.PortCloser, error) {
			panic("spiOpenFn called but test did not stub it")
		}
	}
	if pinFn == nil {
		pinFn = func(string) gpio.PinIO { panic("gpioByNameFn called but test did not stub it") }
	}
	origHost, origOpen, origPin := hostInitFn, spiOpenFn, gpioByNameFn
	hostInitFn = hostFn
	spiOpenFn = openFn
	gpioByNameFn = pinFn
	t.Cleanup(func() {
		hostInitFn = origHost
		spiOpenFn = origOpen
		gpioByNameFn = origPin
	})
	body()
}

// recordingPortCloser wraps spitest.Record with a controllable Close
// behaviour and a Connect that can be made to fail. It records closure
// so tests can assert that error paths clean up the SPI port.
type recordingPortCloser struct {
	spitest.Record
	closeErr   error
	connectErr error
	closed     bool
}

func (r *recordingPortCloser) Close() error {
	r.closed = true
	return r.closeErr
}

func (r *recordingPortCloser) LimitSpeed(_ physic.Frequency) error {
	return nil
}

func (r *recordingPortCloser) Connect(f physic.Frequency, m spi.Mode, bits int) (spi.Conn, error) {
	if r.connectErr != nil {
		return nil, r.connectErr
	}
	return r.Record.Connect(f, m, bits)
}

// pinSet bundles the four GPIO pins required by spiHardware so a
// single helper can build it and dependent test cases can mutate
// individual pins (e.g. swap PWR with a failing pin).
type pinSet struct {
	rst, dc, busy, pwr gpio.PinIO
}

func defaultPinSet() pinSet {
	return pinSet{
		rst:  &gpiotest.Pin{N: "GPIO17", Num: 17},
		dc:   &gpiotest.Pin{N: "GPIO25", Num: 25},
		busy: &gpiotest.Pin{N: "GPIO24", Num: 24},
		pwr:  &gpiotest.Pin{N: "GPIO18", Num: 18},
	}
}

// pinResolver returns a gpioByNameFn replacement that returns the
// matching pin from set for each canonical BCM name and nil otherwise.
// Setting any field of set to nil simulates "pin not found" for that
// name, which is how tests exercise the not-found branches.
func pinResolver(set pinSet) func(string) gpio.PinIO {
	return func(name string) gpio.PinIO {
		switch name {
		case pinNameRST:
			return set.rst
		case pinNameDC:
			return set.dc
		case pinNameBUSY:
			return set.busy
		case pinNamePWR:
			return set.pwr
		}
		return nil
	}
}

// failInPin wraps gpiotest.Pin and fails on In(). Used to exercise the
// busy-pin configure error branch in initRealHardware.
type failInPin struct {
	gpiotest.Pin
	err error
}

func (p *failInPin) In(_ gpio.Pull, _ gpio.Edge) error { return p.err }

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
	// With the hardware tag, createBackend("spi") must reach
	// initRealHardware. Inject a failing host init so the test is
	// deterministic on every host (including non-Pi CI runners).
	wantErr := errors.New("host init boom")
	withInjections(t,
		func() error { return wantErr },
		nil, // spiOpenFn must not be reached when host init fails.
		nil, // gpioByNameFn must not be reached when host init fails.
		func() {
			cfg := &Config{Backend: "spi"}
			profile := &Waveshare7in5V2
			_, err := createBackend(cfg, profile)
			if !errors.Is(err, wantErr) {
				t.Fatalf("createBackend error = %v, want to contain %v", err, wantErr)
			}
		})
}

func TestNewApp_SPI_WithHardwareTag(t *testing.T) {
	wantErr := errors.New("host init boom")
	withInjections(t,
		func() error { return wantErr },
		nil,
		nil,
		func() {
			cfg, err := LoadConfig(strings.NewReader(`
display: waveshare_7in5_v2
backend: spi
`))
			if err != nil {
				t.Fatalf("LoadConfig: %v", err)
			}
			_, err = NewApp(cfg)
			if !errors.Is(err, wantErr) {
				t.Fatalf("NewApp error = %v, want to contain %v", err, wantErr)
			}
		})
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

func TestInitRealHardware_Success(t *testing.T) {
	port := &recordingPortCloser{}
	pins := defaultPinSet()
	withInjections(t,
		func() error { return nil },
		func(name string) (spi.PortCloser, error) {
			if name != spiPortName {
				t.Errorf("spi open name = %q, want %q", name, spiPortName)
			}
			return port, nil
		},
		pinResolver(pins),
		func() {
			hw, err := NewSPIHardware()
			if err != nil {
				t.Fatalf("NewSPIHardware: %v", err)
			}
			if hw.spiConn == nil {
				t.Error("spiConn was not assigned")
			}
			if hw.port != port {
				t.Error("port was not assigned to the opened port")
			}
			if pwr, ok := pins.pwr.(*gpiotest.Pin); ok && pwr.L != gpio.High {
				t.Errorf("PWR pin = %v after init, want High", pwr.L)
			}
			if port.closed {
				t.Error("port was closed on the success path")
			}
		})
}

func TestInitRealHardware_HostInitFails(t *testing.T) {
	wantErr := errors.New("driverreg boom")
	withInjections(t,
		func() error { return wantErr },
		nil,
		nil,
		func() {
			_, err := NewSPIHardware()
			if !errors.Is(err, wantErr) {
				t.Fatalf("err = %v, want %v", err, wantErr)
			}
		})
}

func TestInitRealHardware_SPIOpenFails(t *testing.T) {
	wantErr := errors.New("no spi port")
	withInjections(t,
		func() error { return nil },
		func(string) (spi.PortCloser, error) { return nil, wantErr },
		nil, // GPIO lookup must not be reached when spi open fails.
		func() {
			_, err := NewSPIHardware()
			if !errors.Is(err, wantErr) {
				t.Fatalf("err = %v, want %v", err, wantErr)
			}
		})
}

func TestInitRealHardware_SPIConnectFails(t *testing.T) {
	wantErr := errors.New("connect boom")
	port := &recordingPortCloser{connectErr: wantErr}
	withInjections(t,
		func() error { return nil },
		func(string) (spi.PortCloser, error) { return port, nil },
		pinResolver(defaultPinSet()),
		func() {
			_, err := NewSPIHardware()
			if !errors.Is(err, wantErr) {
				t.Fatalf("err = %v, want %v", err, wantErr)
			}
			if !port.closed {
				t.Error("port was not closed after connect failure")
			}
		})
}

func TestInitRealHardware_PinNotFound(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*pinSet)
		missing string
	}{
		{"rst", func(s *pinSet) { s.rst = nil }, pinNameRST},
		{"dc", func(s *pinSet) { s.dc = nil }, pinNameDC},
		{"busy", func(s *pinSet) { s.busy = nil }, pinNameBUSY},
		{"pwr", func(s *pinSet) { s.pwr = nil }, pinNamePWR},
	} {
		t.Run(tc.name, func(t *testing.T) {
			port := &recordingPortCloser{}
			pins := defaultPinSet()
			tc.mutate(&pins)
			withInjections(t,
				func() error { return nil },
				func(string) (spi.PortCloser, error) { return port, nil },
				pinResolver(pins),
				func() {
					_, err := NewSPIHardware()
					if err == nil {
						t.Fatal("expected error from initRealHardware")
					}
					if !strings.Contains(err.Error(), tc.missing) {
						t.Errorf("err %v does not mention missing pin %q", err, tc.missing)
					}
					if !port.closed {
						t.Error("port was not closed after pin lookup failure")
					}
				})
		})
	}
}

func TestInitRealHardware_PWROutFails(t *testing.T) {
	wantErr := errors.New("pwr fail")
	port := &recordingPortCloser{}
	pins := defaultPinSet()
	pins.pwr = &failOutPin{Pin: gpiotest.Pin{N: "GPIO18"}, err: wantErr}
	withInjections(t,
		func() error { return nil },
		func(string) (spi.PortCloser, error) { return port, nil },
		pinResolver(pins),
		func() {
			_, err := NewSPIHardware()
			if !errors.Is(err, wantErr) {
				t.Fatalf("err = %v, want %v", err, wantErr)
			}
			if !port.closed {
				t.Error("port was not closed after PWR.Out failure")
			}
		})
}

func TestInitRealHardware_BusyInFails(t *testing.T) {
	wantErr := errors.New("busy in fail")
	port := &recordingPortCloser{}
	pins := defaultPinSet()
	pins.busy = &failInPin{Pin: gpiotest.Pin{N: "GPIO24"}, err: wantErr}
	withInjections(t,
		func() error { return nil },
		func(string) (spi.PortCloser, error) { return port, nil },
		pinResolver(pins),
		func() {
			_, err := NewSPIHardware()
			if !errors.Is(err, wantErr) {
				t.Fatalf("err = %v, want %v", err, wantErr)
			}
			if !port.closed {
				t.Error("port was not closed after BUSY.In failure")
			}
			if pwr, ok := pins.pwr.(*gpiotest.Pin); ok && pwr.L != gpio.Low {
				t.Errorf("PWR pin = %v after BUSY.In failure, want Low", pwr.L)
			}
		})
}

func TestSPIHardware_Close_PortCloseError(t *testing.T) {
	portErr := errors.New("port close fail")
	port := &recordingPortCloser{closeErr: portErr}
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
