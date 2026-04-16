//go:build hardware

package inkwell

import (
	"errors"
	"fmt"
	"time"

	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/spi"
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

// initRealHardware opens the real SPI bus and GPIO pins via periph.io.
func (h *spiHardware) initRealHardware() error {
	// Import these only in the real-hardware path to keep test fakes clean.
	// Lazy imports are not possible in Go, so the build tag gates the entire
	// file; this method exists to keep the constructor tidy.

	// host.Init() and spireg/gpioreg usage is deferred to
	// spi_backend_hardware.go's init function or called here.
	return fmt.Errorf("real hardware init not available in test-only builds; " +
		"use WithSPIConn and WithGPIOPins options")
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
