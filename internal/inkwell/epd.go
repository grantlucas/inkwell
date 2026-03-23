package inkwell

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
