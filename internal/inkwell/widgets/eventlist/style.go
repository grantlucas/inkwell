package eventlist

import (
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daydata"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/drawkit"
)

// Style is one of the shapes the day screens list events in, as the
// event-list widget's style setting names it. A screen, or a placed
// event-list widget, picks one and fills in only what its settings
// choose: how many events, whether locations show, and the zone times
// are written in. Everything that makes the shape (the layout, the sizes,
// the gaps, what an empty day says) is the style's, so two places listing
// in the same shape cannot drift apart.
type Style int

const (
	// StackedStyle is bold-five's column: a body-size bold time above
	// a title of up to two lines, 8 px between events, and "--" centred
	// on an empty day.
	StackedStyle Style = iota
	// LargeStyle is today-hero's agenda: the time at twice body size
	// with the title tucked a body ascent under it (clock labels have no
	// descenders to clear), a title of up to two lines, and a hairline
	// across the 20 px between events so a wrapped title does not run
	// into the next large time.
	LargeStyle
	// InlineStyle is row-agenda's and today-hero's day rows: the time
	// and the title on one line, the title past a column as wide as the
	// widest time.
	InlineStyle
)

// StyleNames are the names a config gives each style, in Style order.
var StyleNames = daydata.Names[Style]{"stacked", "large", "inline"}

// String is the style's config name.
func (s Style) String() string { return StyleNames.Name(s) }

const (
	// titleLines caps a stacked title so one long summary cannot
	// eat the whole list.
	titleLines = 2
	// stackedGap separates one event from the next in a column. Events
	// are variable height, so a fixed grid would either waste the short
	// ones or clip the long ones.
	stackedGap = 8
	// largeTimeScale is the large style's time: what you scan for. The
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

// List is the style filled in with a widget's choices. maxEvents below
// 1 lists every event that fits. loc is the zone event clock labels are
// written in: a parsed Event.Start is a correct instant but carries
// whatever zone its feed serialized it with, so formatting it directly
// would leak that zone onto the panel. It must never be nil.
func (s Style) List(maxEvents int, showLocation bool, loc *time.Location) List {
	l := List{MaxEvents: maxEvents, ShowLocation: showLocation, Location: loc}
	switch s {
	case StackedStyle:
		l.TitleLines, l.Gap = titleLines, stackedGap
		l.Empty = Note{Text: emptyDash, Centred: true}
	case LargeStyle:
		l.TitleLines, l.Gap, l.Rules = titleLines, largeGap, true
		l.TimeScale, l.TitleLead = largeTimeScale, drawkit.BodyAscent()
		l.Empty = Note{Text: NothingScheduled}
	default:
		l.Layout = Inline
		l.Empty = Note{Text: NothingScheduled}
	}
	return l
}
