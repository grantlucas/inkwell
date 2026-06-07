//go:build hardware

package inkwell

import (
	"errors"
	"fmt"
	"time"

	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
	"periph.io/x/conn/v3/physic"
	"periph.io/x/conn/v3/spi"
	"periph.io/x/conn/v3/spi/spireg"
	"periph.io/x/host/v3"
)

// Pin and bus identifiers for the Waveshare E-Paper Driver HAT
// connected to a Raspberry Pi via the 40-pin GPIO header. BCM pin
// numbers match the Python reference driver (epdconfig.py) and the
// Waveshare wiki. CS is wired directly to SPI CE0; SendCommand /
// SendData do not toggle it manually because periph.io's spi.Conn
// drives CS as part of every Tx.
const (
	spiPortName = "/dev/spidev0.0"
	spiSpeed    = 4 * physic.MegaHertz

	pinNameRST  = "GPIO17" // active-low reset
	pinNameDC   = "GPIO25" // data (HIGH) / command (LOW)
	pinNameBUSY = "GPIO24" // HIGH = idle, LOW = busy
	pinNamePWR  = "GPIO18" // HIGH = panel powered on
)

// Injection seams for the periph.io entry points. Tests on non-Pi
// hosts override these to avoid touching real device nodes; the
// defaults call straight into periph.io.
//
// These are package-level mutables for symmetry with createSPIBackendFn
// in app.go. They are NOT goroutine-safe — tests that swap them must
// not run with t.Parallel(), and production code must treat them as
// read-only after init.
var (
	hostInitFn = func() error {
		_, err := host.Init()
		return err
	}
	spiOpenFn    = spireg.Open
	gpioByNameFn = gpioreg.ByName
)

// spiHardware drives a real e-paper display via SPI and GPIO using periph.io.
type spiHardware struct {
	spiConn spi.Conn
	port    spi.PortCloser
	rstPin  gpio.PinIO
	dcPin   gpio.PinIO
	busyPin gpio.PinIO
	pwrPin  gpio.PinIO
}

// SPIOption configures an spiHardware instance.
type SPIOption func(*spiHardware)

// WithSPIConn injects a pre-configured SPI connection (for testing).
func WithSPIConn(conn spi.Conn) SPIOption {
	return func(h *spiHardware) {
		h.spiConn = conn
	}
}

// WithGPIOPins injects GPIO pins (for testing).
func WithGPIOPins(rst, dc, busy, pwr gpio.PinIO) SPIOption {
	return func(h *spiHardware) {
		h.rstPin = rst
		h.dcPin = dc
		h.busyPin = busy
		h.pwrPin = pwr
	}
}

// NewSPIHardware creates a hardware backend. Without functional options it
// initialises periph.io and opens the real SPI bus and GPIO pins.
func NewSPIHardware(opts ...SPIOption) (*spiHardware, error) {
	h := &spiHardware{}
	for _, o := range opts {
		o(h)
	}

	// When no options provided, initialise real hardware.
	if h.spiConn == nil && h.port == nil {
		if err := h.initRealHardware(); err != nil {
			return nil, err
		}
	}

	if err := h.validate(); err != nil {
		return nil, err
	}

	return h, nil
}

func (h *spiHardware) validate() error {
	if h.spiConn == nil {
		return fmt.Errorf("spi connection is required")
	}
	if h.rstPin == nil || h.dcPin == nil || h.busyPin == nil || h.pwrPin == nil {
		return fmt.Errorf("all GPIO pins (rst, dc, busy, pwr) are required")
	}
	return nil
}

// initRealHardware brings up periph.io drivers, opens the SPI port at
// /dev/spidev0.0 (4 MHz, mode 0, 8-bit), resolves the four GPIO pins
// the HAT exposes, and powers the panel on. The Waveshare reference
// driver leaves CS to be driven implicitly by the SPI peripheral; we
// do the same — there is no SendCommand/SendData toggle of CS.
func (h *spiHardware) initRealHardware() error {
	if err := hostInitFn(); err != nil {
		return fmt.Errorf("periph host init: %w", err)
	}

	port, err := spiOpenFn(spiPortName)
	if err != nil {
		return fmt.Errorf("open spi port %s: %w", spiPortName, err)
	}

	conn, err := port.Connect(spiSpeed, spi.Mode0, 8)
	if err != nil {
		_ = port.Close()
		return fmt.Errorf("configure spi port: %w", err)
	}

	rst, err := resolvePin(pinNameRST)
	if err != nil {
		_ = port.Close()
		return err
	}
	dc, err := resolvePin(pinNameDC)
	if err != nil {
		_ = port.Close()
		return err
	}
	busy, err := resolvePin(pinNameBUSY)
	if err != nil {
		_ = port.Close()
		return err
	}
	pwr, err := resolvePin(pinNamePWR)
	if err != nil {
		_ = port.Close()
		return err
	}

	if err := pwr.Out(gpio.High); err != nil {
		_ = port.Close()
		return fmt.Errorf("pwr pin high: %w", err)
	}
	if err := busy.In(gpio.PullNoChange, gpio.NoEdge); err != nil {
		_ = port.Close()
		return fmt.Errorf("busy pin in: %w", err)
	}

	h.port = port
	h.spiConn = conn
	h.rstPin = rst
	h.dcPin = dc
	h.busyPin = busy
	h.pwrPin = pwr
	return nil
}

func resolvePin(name string) (gpio.PinIO, error) {
	if p := gpioByNameFn(name); p != nil {
		return p, nil
	}
	return nil, fmt.Errorf("gpio pin %s not found (is the kernel driver loaded?)", name)
}

func (h *spiHardware) SendCommand(cmd byte) error {
	if err := h.dcPin.Out(gpio.Low); err != nil {
		return fmt.Errorf("dc pin low: %w", err)
	}
	return h.spiConn.Tx([]byte{cmd}, nil)
}

func (h *spiHardware) SendData(data []byte) error {
	if err := h.dcPin.Out(gpio.High); err != nil {
		return fmt.Errorf("dc pin high: %w", err)
	}
	return h.spiConn.Tx(data, nil)
}

func (h *spiHardware) ReadBusy() bool {
	return h.busyPin.Read() == gpio.High
}

func (h *spiHardware) Reset() error {
	if err := h.rstPin.Out(gpio.High); err != nil {
		return fmt.Errorf("rst high: %w", err)
	}
	time.Sleep(20 * time.Millisecond)

	if err := h.rstPin.Out(gpio.Low); err != nil {
		return fmt.Errorf("rst low: %w", err)
	}
	time.Sleep(2 * time.Millisecond)

	if err := h.rstPin.Out(gpio.High); err != nil {
		return fmt.Errorf("rst high: %w", err)
	}
	time.Sleep(20 * time.Millisecond)

	return nil
}

func (h *spiHardware) Close() error {
	var errs []error

	if err := h.pwrPin.Out(gpio.Low); err != nil {
		errs = append(errs, fmt.Errorf("pwr pin low: %w", err))
	}

	if h.port != nil {
		if err := h.port.Close(); err != nil {
			errs = append(errs, fmt.Errorf("spi port close: %w", err))
		}
	}

	return errors.Join(errs...)
}

// Compile-time interface check.
var _ Hardware = (*spiHardware)(nil)
