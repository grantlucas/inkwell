package inkwell

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
