package eventlist

import (
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/widgets/drawkit"
)

// Preset is one of the shapes the day screens list events in. A screen,
// or a placed event-list widget, picks one and fills in only what its
// settings choose: how many events, whether locations show, and the zone
// times are written in. Everything that makes the shape (the layout, the
// sizes, the gaps, what an empty day says) is the preset's, so two
// places listing in the same shape cannot drift apart.
type Preset int

const (
	// PresetStacked is bold-five's column: a body-size bold time above
	// a title of up to two lines, 8 px between events, and "--" centred
	// on an empty day.
	PresetStacked Preset = iota
	// PresetLarge is today-hero's agenda: the time at twice body size
	// with the title tucked a body ascent under it (clock labels have no
	// descenders to clear), a title of up to two lines, and a hairline
	// across the 20 px between events so a wrapped title does not run
	// into the next large time.
	PresetLarge
	// PresetInline is row-agenda's and today-hero's day rows: the time
	// and the title on one line, the title past a column as wide as the
	// widest time.
	PresetInline
)

// presetNames are the names a config gives each preset, in Preset order.
var presetNames = []string{"stacked", "large", "inline"}

// PresetNames lists every preset's config name, in a stable order, for
// an error that says what is accepted.
func PresetNames() []string { return append([]string(nil), presetNames...) }

// ParsePreset returns the preset a config names, and whether it names
// one.
func ParsePreset(name string) (Preset, bool) {
	for i, n := range presetNames {
		if n == name {
			return Preset(i), true
		}
	}
	return 0, false
}

// String is the preset's config name.
func (p Preset) String() string { return presetNames[p] }

const (
	// presetTitleLines caps a stacked title so one long summary cannot
	// eat the whole list.
	presetTitleLines = 2
	// stackedGap separates one event from the next in a column. Events
	// are variable height, so a fixed grid would either waste the short
	// ones or clip the long ones.
	stackedGap = 8
	// largeTimeScale is the large preset's time: what you scan for. The
	// title stays body size, because it is what you read once you are
	// close enough to care.
	largeTimeScale = 2
	// largeGap is wide because the next time is drawn at twice body
	// size, and the hairline sits across its middle.
	largeGap = 20
	// emptyDash is what a stacked column says on an empty day, so it does
	// not read as a rendering fault. Centred, because it marks the
	// column rather than starting a line of text.
	emptyDash = "--"
)

// Style is the preset filled in with a widget's choices. maxEvents below
// 1 lists every event that fits. loc is the zone event clock labels are
// written in: a parsed Event.Start is a correct instant but carries
// whatever zone its feed serialized it with, so formatting it directly
// would leak that zone onto the panel. It must never be nil.
func (p Preset) Style(maxEvents int, showLocation bool, loc *time.Location) Style {
	s := Style{MaxEvents: maxEvents, ShowLocation: showLocation, Location: loc}
	switch p {
	case PresetStacked:
		s.TitleLines, s.Gap = presetTitleLines, stackedGap
		s.Empty = Note{Text: emptyDash, Centred: true}
	case PresetLarge:
		s.TitleLines, s.Gap, s.Rules = presetTitleLines, largeGap, true
		s.TimeScale, s.TitleLead = largeTimeScale, drawkit.BodyAscent()
		s.Empty = Note{Text: NothingScheduled}
	default:
		s.Layout = Inline
		s.Empty = Note{Text: NothingScheduled}
	}
	return s
}
