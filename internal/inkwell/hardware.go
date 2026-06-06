package inkwell

import "image"

// Hardware is the low-level SPI/GPIO transport interface.
// Implementations include real SPI hardware (production), a recording mock
// (tests), an image writer (PNG output), and a web preview server.
type Hardware interface {
	// SendCommand sends a single command byte with DC=LOW.
	SendCommand(cmd byte) error

	// SendData sends a data payload with DC=HIGH.
	SendData(data []byte) error

	// ReadBusy returns true when the display is idle (BUSY=HIGH).
	ReadBusy() bool

	// Reset performs the hardware reset sequence (RST pin toggle).
	Reset() error

	// Close releases hardware resources (SPI port, GPIO pins).
	Close() error
}

// FrameSink is an optional capability for Hardware backends that want to see
// the pre-pack source frame in addition to (or instead of) the packed device
// buffer. Backends that render previews use this to display the high-fidelity
// grayscale composition before the packer collapses it to the device's color
// depth. Backends driving real e-ink hardware can ignore this hook.
type FrameSink interface {
	// SetSourceFrame receives the composited frame each cycle, immediately
	// before PackImage is called. Implementations must treat the frame as
	// read-only after the call returns; the caller may reuse the buffer.
	SetSourceFrame(frame *image.Paletted)
}
