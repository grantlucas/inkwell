// Package daybadge is the day badge: the head of one day, naming the day
// and summing up its weather (the condition icon, the high and the low)
// in one of the shapes the full-screen day widgets draw it in. A badge
// is drawn either by a full-screen widget, beside its own chart and
// event list, or placed on a screen on its own as the day-badge widget.
//
// Each style is the badge of one full-screen widget, at the sizes and
// offsets that widget has always drawn it at, so a screen composed from
// placed badges reads exactly like the screen it was taken from.
package daybadge

import (
	"image"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
)

// Style is one of the shapes a day badge is drawn in.
type Style int

const (
	// Column is bold-five's column head: the weekday over the date
	// numeral, centred, then the condition icon on the left with the
	// high at twice body size and the low under it on the right.
	Column Style = iota
	// Row is row-agenda's: the date numeral with the weekday and month
	// stacked beside it, then the condition icon with the high over the
	// low.
	Row
	// Compact is today-hero's day rows': the weekday (or "TOMORROW")
	// over the date numeral, with the condition icon and the high and
	// low on one line beside it.
	Compact
	// Hero is today-hero's hero: the weekday and date at three times
	// body size, the month and, on today, the fuzzy clock, closed off
	// by a rule, then a large condition icon, the high at three times
	// body size, the low and the condition's name.
	Hero
)

// styleNames are the names a config gives each style, in Style order.
var styleNames = []string{"column", "row", "compact", "hero"}

// StyleNames lists every style's config name, in a stable order, for an
// error that says what is accepted.
func StyleNames() []string { return append([]string(nil), styleNames...) }

// ParseStyle returns the style a config names, and whether it names one.
func ParseStyle(name string) (Style, bool) {
	for i, n := range styleNames {
		if n == name {
			return Style(i), true
		}
	}
	return 0, false
}

// String is the style's config name.
func (s Style) String() string { return styleNames[s] }

// Size is the room the style is drawn in: what the full-screen widget it
// comes from gives it. Every element sits at a fixed offset from the
// rect's top-left corner (or, for a right-aligned reading, its right
// edge), so a badge given less room than this would run past its rect.
func (s Style) Size() image.Point {
	switch s {
	case Column:
		return image.Pt(columnW, columnH)
	case Row:
		return image.Pt(rowW, rowH)
	case Compact:
		return image.Pt(compactW, compactH)
	default:
		return image.Pt(heroW, heroH)
	}
}

// Draw draws day's badge into r. now is the dashboard's clock, already in
// the display zone: the compact badge tags tomorrow by it, and the hero
// writes the fuzzy clock from it on today. unit is the temperature unit
// readings are written in.
//
// A day the forecast doesn't reach still names the day but draws no
// weather: a zero would state a temperature nobody forecast. Nothing is
// highlighted on today, which is shown by position (CLAUDE.md).
func (s Style) Draw(frame *image.Paletted, r image.Rectangle, day daygrid.Day, now time.Time, unit string) {
	switch s {
	case Column:
		drawColumn(frame, r, day, unit)
	case Row:
		drawRow(frame, r, day, unit)
	case Compact:
		drawCompact(frame, r, day, now, unit)
	default:
		drawHero(frame, r, day, now, unit)
	}
}
