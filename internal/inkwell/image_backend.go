package inkwell

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
)

// ImageBackend is a Hardware implementation that writes PNG files on each
// display refresh. Useful for visually inspecting output without hardware.
type ImageBackend struct {
	profile     *DisplayProfile
	outputDir   string
	seqNum      int
	lastCmd     byte
	capturedBuf []byte
}

var _ Hardware = (*ImageBackend)(nil)

// NewImageBackend creates an ImageBackend that writes PNGs to outputDir.
func NewImageBackend(profile *DisplayProfile, outputDir string) *ImageBackend {
	return &ImageBackend{
		profile:   profile,
		outputDir: outputDir,
	}
}

// SendCommand tracks the last command sent. On RefreshCmd, writes a PNG
// from the captured buffer.
func (b *ImageBackend) SendCommand(cmd byte) error {
	b.lastCmd = cmd
	if cmd == b.profile.RefreshCmd && b.capturedBuf != nil {
		return b.writePNG()
	}
	return nil
}

// SendData captures the data payload when the previous command was NewBufferCmd.
func (b *ImageBackend) SendData(data []byte) error {
	if b.lastCmd == b.profile.NewBufferCmd {
		b.capturedBuf = make([]byte, len(data))
		copy(b.capturedBuf, data)
	}
	return nil
}

// ReadBusy always returns true (idle) — no real hardware to wait on.
func (b *ImageBackend) ReadBusy() bool { return true }

// Reset is a no-op for the image backend.
func (b *ImageBackend) Reset() error { return nil }

// Close is a no-op for the image backend.
func (b *ImageBackend) Close() error { return nil }

func (b *ImageBackend) writePNG() error {
	img := UnpackBuffer(b.profile, b.capturedBuf)
	name := fmt.Sprintf("frame_%03d.png", b.seqNum)
	b.seqNum++

	f, err := os.Create(filepath.Join(b.outputDir, name))
	if err != nil {
		return err
	}
	defer f.Close()

	return png.Encode(f, img)
}
