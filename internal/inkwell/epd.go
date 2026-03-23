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
// The old buffer is sent inverted (XOR with 0xFF) per the display protocol,
// then the new buffer is sent as-is. After refresh, waits for the display
// to finish updating. Returns an error if the buffer size doesn't match
// the profile's expected BufferSize().
func (d *EPD) Display(buffer []byte) error {
	expected := d.profile.BufferSize()
	if len(buffer) != expected {
		return fmt.Errorf("buffer size %d does not match expected %d", len(buffer), expected)
	}

	// Send old buffer (inverted)
	inverted := make([]byte, len(buffer))
	for i, b := range buffer {
		inverted[i] = ^b
	}
	if err := d.hw.SendCommand(d.profile.OldBufferCmd); err != nil {
		return err
	}
	if err := d.hw.SendData(inverted); err != nil {
		return err
	}

	// Send new buffer
	if err := d.hw.SendCommand(d.profile.NewBufferCmd); err != nil {
		return err
	}
	if err := d.hw.SendData(buffer); err != nil {
		return err
	}

	// Trigger refresh and wait
	if err := d.hw.SendCommand(d.profile.RefreshCmd); err != nil {
		return err
	}
	d.hw.ReadBusy()

	return nil
}

// DisplayPartial updates a rectangular region of the display without
// redrawing the full screen. X is aligned down to a byte boundary (multiple
// of 8) and width is aligned up. The buffer contains the packed pixel data
// for the partial region only. Sends VCOM partial settings, enters partial
// mode, sets the window coordinates, sends data, and triggers refresh.
// Returns an error if the profile doesn't support partial refresh.
func (d *EPD) DisplayPartial(buffer []byte, x, y, w, h int) error {
	if !d.profile.Capabilities.PartialRefresh {
		return fmt.Errorf("display %s does not support partial refresh", d.profile.Name)
	}

	// Align x down and width up to byte boundaries (multiples of 8)
	alignedX := (x / 8) * 8
	alignedW := ((w + (x - alignedX) + 7) / 8) * 8

	xEnd := alignedX + alignedW - 1
	yEnd := y + h - 1

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
		byte(y >> 8), byte(y & 0xFF), // Y start
		byte(yEnd >> 8), byte(yEnd & 0xFF), // Y end
		0x01, // Scan inside partial window
	}
	if err := d.hw.SendData(windowData); err != nil {
		return err
	}

	// Send partial buffer data
	if err := d.hw.SendCommand(d.profile.NewBufferCmd); err != nil {
		return err
	}
	if err := d.hw.SendData(buffer); err != nil {
		return err
	}

	// Trigger refresh and wait
	if err := d.hw.SendCommand(d.profile.RefreshCmd); err != nil {
		return err
	}
	d.hw.ReadBusy()

	return nil
}

// Clear sets the entire display to white by sending an all-0xFF buffer
// through the normal Display path (which handles inversion automatically).
func (d *EPD) Clear() error {
	buf := make([]byte, d.profile.BufferSize())
	for i := range buf {
		buf[i] = 0xFF
	}
	return d.Display(buf)
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
		if cmd.Data != nil {
			if err := d.hw.SendData(cmd.Data); err != nil {
				return err
			}
		} else {
			d.hw.ReadBusy()
		}
	}
	return nil
}
