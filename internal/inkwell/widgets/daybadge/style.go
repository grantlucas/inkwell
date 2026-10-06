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

	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daydata"
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

// StyleNames are the names a config gives each style, in Style order.
var StyleNames = daydata.Names[Style]{"column", "row", "compact", "hero"}

// String is the style's config name.
func (s Style) String() string { return StyleNames.Name(s) }

// shape is what a style is: the room it is drawn in and how it draws.
type shape struct {
	size image.Point
	draw func(frame *image.Paletted, r image.Rectangle, day daydata.Day, now time.Time, unit string)
}

// shapes are the styles' shapes, in Style order.
var shapes = [...]shape{
	Column:  {image.Pt(columnW, columnH), drawColumn},
	Row:     {image.Pt(rowW, rowH), drawRow},
	Compact: {image.Pt(compactW, compactH), drawCompact},
	Hero:    {image.Pt(heroW, heroH), drawHero},
}

// Size is the room the style is drawn in: what the full-screen widget it
// comes from gives it. Every element sits at a fixed offset from the
// rect's top-left corner (or, for a right-aligned reading, its right
// edge), so a badge given less room than this would run past its rect.
func (s Style) Size() image.Point { return shapes[s].size }

// Draw draws day's badge into r. now is the dashboard's clock, already in
// the display zone: the compact badge tags tomorrow by it, and the hero
// writes the fuzzy clock from it on today. unit is the temperature unit
// readings are written in.
//
// A day the forecast doesn't reach still names the day but draws no
// weather: a zero would state a temperature nobody forecast. Nothing is
// highlighted on today, which is shown by position (CLAUDE.md).
func (s Style) Draw(frame *image.Paletted, r image.Rectangle, day daydata.Day, now time.Time, unit string) {
	shapes[s].draw(frame, r, day, now, unit)
}
