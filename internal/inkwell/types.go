// Package inkwell provides a data-driven e-ink display driver framework.
package inkwell

import "fmt"

// Command is a single SPI command and its data payload.
type Command struct {
	Reg  byte   // Command register (sent with DC=LOW)
	Data []byte // Parameter bytes (sent with DC=HIGH), nil if none
}

// ColorDepth describes how pixels are packed into the buffer.
type ColorDepth int

const (
	// BW is 1 bit per pixel, 8 pixels per byte.
	BW ColorDepth = iota
	// Gray4 is 2 bits per pixel, 4 pixels per byte.
	Gray4
	// Color7 is 4 bits per pixel, 2 pixels per byte.
	Color7
)

// String returns the name of the ColorDepth.
func (c ColorDepth) String() string {
	switch c {
	case BW:
		return "BW"
	case Gray4:
		return "Gray4"
	case Color7:
		return "Color7"
	default:
		return fmt.Sprintf("ColorDepth(%d)", int(c))
	}
}

// Capabilities flags for what a display supports.
type Capabilities struct {
	FastRefresh    bool
	PartialRefresh bool
	Grayscale      bool
}

// InitMode selects which initialization sequence to use.
type InitMode int

const (
	// InitFull is the standard full refresh mode.
	InitFull InitMode = iota
	// InitFast is a fast full refresh mode (if supported).
	InitFast
	// InitPartial is the partial refresh mode (if supported).
	InitPartial
	// Init4Gray is the 4-level grayscale mode (if supported).
	Init4Gray
)

// Region describes a rectangular area on the display for partial updates.
type Region struct {
	X, Y int // Top-left corner (X will be byte-aligned by the driver)
	W, H int // Width and height in pixels
}

// String returns the name of the InitMode.
func (m InitMode) String() string {
	switch m {
	case InitFull:
		return "InitFull"
	case InitFast:
		return "InitFast"
	case InitPartial:
		return "InitPartial"
	case Init4Gray:
		return "Init4Gray"
	default:
		return fmt.Sprintf("InitMode(%d)", int(m))
	}
}
