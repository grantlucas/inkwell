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
