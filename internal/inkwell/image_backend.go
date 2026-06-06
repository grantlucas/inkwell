package inkwell

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
)

// ImageBackend is a Hardware implementation that writes PNG files on each
// display refresh. Useful for visually inspecting output without hardware.
// Captures both wire planes so Gray4 frames reconstruct correctly — for
// BW the old plane is redundant and ignored downstream.
type ImageBackend struct {
	profile     *DisplayProfile
	outputDir   string
	seqNum      int
	lastCmd     byte
	capturedOld []byte
	capturedNew []byte
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
// from the captured planes.
func (b *ImageBackend) SendCommand(cmd byte) error {
	if b.profile == nil {
		return fmt.Errorf("image backend: nil display profile")
	}
	b.lastCmd = cmd
	if cmd == b.profile.RefreshCmd && b.capturedNew != nil {
		return b.writePNG()
	}
	return nil
}

// SendData captures the data payload after either buffer-write command.
// Both planes are kept so the Gray4 reconstruction has what it needs;
// the BW path simply ignores the old plane.
func (b *ImageBackend) SendData(data []byte) error {
	if b.profile == nil {
		return fmt.Errorf("image backend: nil display profile")
	}
	switch b.lastCmd {
	case b.profile.OldBufferCmd:
		b.capturedOld = append(b.capturedOld[:0], data...)
	case b.profile.NewBufferCmd:
		b.capturedNew = append(b.capturedNew[:0], data...)
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
	img, err := reconstructFrame(b.profile, b.capturedOld, b.capturedNew)
	if err != nil {
		return fmt.Errorf("image backend: %w", err)
	}
	name := fmt.Sprintf("frame_%03d.png", b.seqNum)

	path := filepath.Join(b.outputDir, name)
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("image backend: create %s: %w", path, err)
	}
	defer f.Close()

	b.seqNum++
	return png.Encode(f, img)
}
