package inkwell

import "fmt"

// EPD is the generic e-ink display driver, parameterized by a DisplayProfile.
// It interprets profile data to drive any supported display through a Hardware backend.
type EPD struct {
	hw      Hardware
	profile *DisplayProfile
}

// NewEPD creates a new EPD driver for the given hardware backend and display profile.
func NewEPD(hw Hardware, profile *DisplayProfile) *EPD {
	return &EPD{hw: hw, profile: profile}
}

// Width returns the display width in pixels from the profile.
func (d *EPD) Width() int { return d.profile.Width }

// Height returns the display height in pixels from the profile.
func (d *EPD) Height() int { return d.profile.Height }

// Init initializes the display for the given mode by performing a hardware
// reset and then executing the profile's init sequence for that mode.
// Returns an error if the profile does not support the requested mode.
func (d *EPD) Init(mode InitMode) error {
	if err := d.hw.Reset(); err != nil {
		return err
	}

	var seq []Command
	switch mode {
	case InitFull:
		seq = d.profile.InitFull
	case InitFast:
		seq = d.profile.InitFast
	case InitPartial:
		seq = d.profile.InitPartial
	case Init4Gray:
		seq = d.profile.Init4Gray
	}

	if seq == nil {
		return fmt.Errorf("display %s does not support mode %v", d.profile.Name, mode)
	}

	return d.execSequence(seq)
}

// Display sends a full frame buffer to the display and triggers a refresh.
// The exact wire protocol depends on the profile's ColorDepth:
//
//   - BW: the buffer is sent inverted (XOR with 0xFF) to OldBufferCmd and
//     as-is to NewBufferCmd — the controller's 1-bit refresh expects the
//     "previous" plane inverted relative to the "new" one.
//   - Gray4: the 2bpp buffer is split into two 1bpp planes (low bits to
//     OldBufferCmd, high bits to NewBufferCmd) per the upstream Waveshare
//     EPD_4Gray_Display protocol. The XOR inversion does not apply here —
//     it's a BW-specific trick.
//
// After refresh, waits for the display to finish updating. Returns an
// error if the buffer size doesn't match the profile's BufferSize() or
// the profile uses a ColorDepth without a wire protocol implementation.
func (d *EPD) Display(buffer []byte) error {
	expected := d.profile.BufferSize()
	if len(buffer) != expected {
		return fmt.Errorf("buffer size %d does not match expected %d", len(buffer), expected)
	}

	var oldData, newData []byte
	switch d.profile.Color {
	case BW:
		oldData = make([]byte, len(buffer))
		for i, b := range buffer {
			oldData[i] = ^b
		}
		newData = buffer
	case Gray4:
		oldData, newData = splitGray4Planes(buffer)
	default:
		return fmt.Errorf("Display: unsupported color depth %v", d.profile.Color)
	}

	if err := d.hw.SendCommand(d.profile.OldBufferCmd); err != nil {
		return err
	}
	if err := d.hw.SendData(oldData); err != nil {
		return err
	}

	if err := d.hw.SendCommand(d.profile.NewBufferCmd); err != nil {
		return err
	}
	if err := d.hw.SendData(newData); err != nil {
		return err
	}

	if err := d.hw.SendCommand(d.profile.RefreshCmd); err != nil {
		return err
	}
	d.hw.ReadBusy()

	return nil
}

// DisplayPartial updates a rectangular region of the display without
// redrawing the full screen. The region's X is aligned down to a byte
// boundary (multiple of 8) and width is aligned up. newBuf and oldBuf
// contain the packed pixel data for the partial region only — newBuf is
// the frame to show, oldBuf is the frame currently on the panel.
//
// Both planes must be written: the 7.5" V2 controller computes which
// pixels to flip by diffing OldBufferCmd against NewBufferCmd, but a
// partial refresh never repopulates the old plane on its own. Feeding it
// the previous frame each time keeps the diff correct — otherwise stale
// or degraded controller RAM produces visible noise during partial
// updates.
//
// Returns an error if the profile doesn't support partial refresh.
func (d *EPD) DisplayPartial(newBuf, oldBuf []byte, region Region) error {
	if !d.profile.Capabilities.PartialRefresh {
		return fmt.Errorf("display %s does not support partial refresh", d.profile.Name)
	}

	if err := d.sendPartialWindow(region); err != nil {
		return err
	}

	// Resync the controller's "old" plane with the frame on the panel.
	if err := d.hw.SendCommand(d.profile.OldBufferCmd); err != nil {
		return err
	}
	if err := d.hw.SendData(oldBuf); err != nil {
		return err
	}

	// Send the new partial buffer data.
	if err := d.hw.SendCommand(d.profile.NewBufferCmd); err != nil {
		return err
	}
	if err := d.hw.SendData(newBuf); err != nil {
		return err
	}

	// Trigger refresh and wait
	if err := d.hw.SendCommand(d.profile.RefreshCmd); err != nil {
		return err
	}
	d.hw.ReadBusy()

	return nil
}

// alignRegionX returns the region's X start aligned down to a byte boundary
// (multiple of 8) and its width aligned up so the span covers whole bytes.
func alignRegionX(region Region) (alignedX, alignedW int) {
	alignedX = (region.X / 8) * 8
	alignedW = ((region.W + (region.X - alignedX) + 7) / 8) * 8
	return alignedX, alignedW
}

// sendPartialWindow configures the display controller for a partial update:
// sets VCOM interval, enters partial mode, and defines the update window.
func (d *EPD) sendPartialWindow(region Region) error {
	alignedX, alignedW := alignRegionX(region)

	xEnd := alignedX + alignedW - 1
	yEnd := region.Y + region.H - 1

	// Set VCOM interval for partial mode
	if err := d.hw.SendCommand(0x50); err != nil {
		return err
	}
	if err := d.hw.SendData(d.profile.PartialVCOM); err != nil {
		return err
	}

	// Enter partial mode
	if err := d.hw.SendCommand(d.profile.PartialEnterCmd); err != nil {
		return err
	}

	// Set partial window coordinates (9 bytes)
	if err := d.hw.SendCommand(d.profile.PartialWindowCmd); err != nil {
		return err
	}
	windowData := []byte{
		byte(alignedX >> 8), byte(alignedX & 0xFF), // X start
		byte(xEnd >> 8), byte(xEnd & 0xFF), // X end
		byte(region.Y >> 8), byte(region.Y & 0xFF), // Y start
		byte(yEnd >> 8), byte(yEnd & 0xFF), // Y end
		0x01, // Scan inside partial window
	}
	return d.hw.SendData(windowData)
}

// Clear sets the entire display to white. The all-white sentinel is the
// zero byte in both supported encodings: packBW sets bit 1 for black
// (so 0=white), and packGray4 codes white as 00 (so the byte 0x00 is
// four white pixels). Passing a zero-filled buffer through Display
// drives the correct planes on the wire for either color depth.
func (d *EPD) Clear() error {
	return d.Display(make([]byte, d.profile.BufferSize()))
}

// Sleep puts the display into deep sleep mode by executing the profile's
// sleep sequence (VCOM setting, power off, deep sleep command).
func (d *EPD) Sleep() error {
	return d.execSequence(d.profile.SleepSequence)
}

// Close puts the display to sleep and then releases hardware resources.
func (d *EPD) Close() error {
	if err := d.Sleep(); err != nil {
		return err
	}
	return d.hw.Close()
}

// execSequence sends a series of commands to the display. Commands with a
// non-nil Data payload send command + data. Commands with nil Data send
// just the command byte and then wait for the display to become idle
// (e.g., Power On requires a busy wait afterward).
func (d *EPD) execSequence(seq []Command) error {
	for _, cmd := range seq {
		if err := d.hw.SendCommand(cmd.Reg); err != nil {
			return err
		}
		if cmd.Data == nil {
			d.hw.ReadBusy()
			continue
		}
		if err := d.hw.SendData(cmd.Data); err != nil {
			return err
		}
	}
	return nil
}
