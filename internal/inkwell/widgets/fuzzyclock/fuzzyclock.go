// Package fuzzyclock implements a widget that renders the current time as
// natural-language English ("half past eight", "about half past five",
// "just after half past two") for e-ink displays.
//
// Unlike the precise clock widget, a fuzzy clock only changes meaningfully
// every ~5 minutes, which makes it the prototypical "low-flash" widget: pair
// it with a slow per-widget refresh cadence (e.g. refresh: "5m") and the panel
// stays quiet while the time stays glanceable.
//
// The rendered string is a pure, deterministic function of the time and the
// configured options (see Phrase): the same minute always produces the same
// string, so the widget never surprises the refresh queue with an unexpected
// change.
package fuzzyclock

import (
	"fmt"
	"image"
	"strings"
	"time"

	"golang.org/x/image/font"

	"github.com/grantlucas/inkwell/internal/inkwell/fonts"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/drawkit"
)

// Compile-time interface check.
var _ widget.Widget = (*Widget)(nil)

var fuzzyFace font.Face

func init() {
	fuzzyFace = mustLoadFuzzyFace()
}

// mustLoadFuzzyFace is extracted so the font-load panic branch is reachable
// from tests via fonts.SwapDataForTest. Production paths invoke it once at
// init time.
func mustLoadFuzzyFace() font.Face {
	f, err := fonts.Face(fonts.Bold, 16)
	if err != nil {
		panic("fuzzy_clock: load font: " + err.Error())
	}
	return f
}

// Style controls the letter casing of a rendered phrase.
type Style int

const (
	StyleSentence Style = iota // "About half past eight" (default)
	StyleTitle                 // "About Half Past Eight"
	StyleLower                 // "about half past eight"
)

// Align controls horizontal alignment of the phrase within the widget bounds.
type Align int

const (
	AlignCenter Align = iota
	AlignLeft
	AlignRight
)

// Options bundles the wording knobs Phrase honors. Every field changes the
// string that comes back; placement is a separate axis (see Align), so a
// caller that only wants the words never has to think about layout. The zero
// value is sentence case, 12-hour, no noon/midnight substitution.
type Options struct {
	Style        Style
	NoonMidnight bool // substitute "noon"/"midnight" for "twelve" (12-hour only)
	Use24Hour    bool // spell the hour as 0..23 instead of 1..12
}

// Widget renders the current time as a natural-language English phrase.
type Widget struct {
	bounds image.Rectangle
	now    func() time.Time
	opts   Options
	align  Align
	scale  int
}

// edgeInset is how far a left- or right-aligned phrase sits from its edge,
// matching the clock widget.
const edgeInset = 4

// New creates a fuzzy clock Widget. A nil now falls back to time.Now so the
// widget renders something reasonable when callers wire it up without an
// explicit clock. The zero Align centers the phrase, preserving the widget's
// original behavior. scale is the integer size multiplier (1 is the original
// size); anything below 1 draws at 1.
//
// New does not check that the phrase fits: Factory does that once, at
// configuration time, so a screen never finds out minute by minute.
func New(bounds image.Rectangle, now func() time.Time, opts Options, align Align, scale int) *Widget {
	if now == nil {
		now = time.Now
	}
	return &Widget{bounds: bounds, now: now, opts: opts, align: align, scale: max(scale, 1)}
}

// Bounds returns the rectangle this widget occupies on the display.
func (w *Widget) Bounds() image.Rectangle { return w.bounds }

// Render draws the fuzzy time within the bounds using black text on a white
// background, aligned per the widget's Align (center by default). The phrase is
// the body face drawn through the drawkit helpers at the widget's scale, so it
// is a solid 1-bit mask at every size. Left/right alignment insets the text 4px
// from the matching edge, matching the clock widget.
func (w *Widget) Render(frame *image.Paletted) error {
	drawkit.FillWhite(frame, w.bounds)

	drawer := drawkit.Scaled(fuzzyFace, w.scale, widget.PaperBlack)
	text := Phrase(w.now(), w.opts)
	textW := drawer.Measure(text)
	metrics := fuzzyFace.Metrics()
	textH := (metrics.Ascent + metrics.Descent).Ceil() * w.scale

	var x int
	switch w.align {
	case AlignLeft:
		x = w.bounds.Min.X + edgeInset
	case AlignRight:
		x = w.bounds.Max.X - textW - edgeInset
	default:
		x = w.bounds.Min.X + (w.bounds.Dx()-textW)/2
	}
	y := w.bounds.Min.Y + (w.bounds.Dy()-textH)/2 + metrics.Ascent.Ceil()*w.scale

	drawer.Draw(frame, x, y, text)
	return nil
}

// fitError reports why the longest phrase this configuration can produce does
// not fit w's bounds, or nil if it does. The widget draws a different phrase
// every five minutes, so checking the one that happens to be showing at load
// time would let a screen silently clip its clock at 8:25 on some later day.
//
// Ink is wider than the advance by the dilation radius on each side, and an
// edge-aligned phrase also gives up its inset, so both count against the room.
func (w *Widget) fitError() error {
	drawer := drawkit.Scaled(fuzzyFace, w.scale, widget.PaperBlack)
	grow := drawer.Grow

	longest := ""
	for hour := range 24 {
		for minute := 0; minute < 60; minute += 5 {
			p := Phrase(time.Date(2000, 1, 1, hour, minute, 0, 0, time.UTC), w.opts)
			if drawer.Measure(p) > drawer.Measure(longest) {
				longest = p
			}
		}
	}

	needW := drawer.Measure(longest) + 2*grow
	if w.align != AlignCenter {
		needW += edgeInset
	}
	metrics := fuzzyFace.Metrics()
	needH := (metrics.Ascent+metrics.Descent).Ceil()*w.scale + 2*grow

	if needW > w.bounds.Dx() || needH > w.bounds.Dy() {
		return fmt.Errorf("fuzzy_clock: %q at scale %d needs %dx%d px but bounds are %dx%d",
			longest, w.scale, needW, needH, w.bounds.Dx(), w.bounds.Dy())
	}
	return nil
}

// Factory creates a fuzzy clock Widget from config and dependencies.
// Supported config keys:
//   - style (string): "sentence" (default), "title", or "lower"
//   - use_words_for_noon_and_midnight (bool): substitute "noon"/"midnight"
//     for "twelve" in 12-hour mode. Default: true.
//   - use_24_hour (bool): spell the hour as 0..23 instead of 1..12.
//     Default: false. The noon/midnight substitution does not apply in
//     24-hour mode.
//   - language (string): only "en" is supported (default). The key is a
//     forward-looking hook for localization; any other value is rejected.
//   - align (string): "center" (default), "left", or "right". Pins the phrase
//     to an edge so a corner placement keeps a fixed anchor as the phrase
//     length changes. Left/right inset 4px.
//   - scale (int): whole-number size multiplier, at least 1. Default: 1, the
//     original size. It is set, not derived from the bounds: auto-fitting would
//     resize the clock whenever the phrase length changed, and every change is
//     a flash on this panel. The longest phrase for the configured style and
//     hour format must fit the bounds at this scale or the config is rejected.
func Factory(bounds image.Rectangle, config map[string]any, deps widget.Deps) (widget.Widget, error) {
	opts := Options{Style: StyleSentence, NoonMidnight: true}
	var align Align

	if v, ok := config["style"]; ok {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("fuzzy_clock: style must be a string, got %T", v)
		}
		switch s {
		case "sentence":
			opts.Style = StyleSentence
		case "title":
			opts.Style = StyleTitle
		case "lower":
			opts.Style = StyleLower
		default:
			return nil, fmt.Errorf("fuzzy_clock: invalid style %q (must be sentence, title, or lower)", s)
		}
	}

	if v, ok := config["use_words_for_noon_and_midnight"]; ok {
		b, ok := v.(bool)
		if !ok {
			return nil, fmt.Errorf("fuzzy_clock: use_words_for_noon_and_midnight must be a bool, got %T", v)
		}
		opts.NoonMidnight = b
	}

	if v, ok := config["use_24_hour"]; ok {
		b, ok := v.(bool)
		if !ok {
			return nil, fmt.Errorf("fuzzy_clock: use_24_hour must be a bool, got %T", v)
		}
		opts.Use24Hour = b
	}

	if v, ok := config["language"]; ok {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("fuzzy_clock: language must be a string, got %T", v)
		}
		if s != "en" {
			return nil, fmt.Errorf("fuzzy_clock: unsupported language %q (only \"en\" is supported)", s)
		}
	}

	if v, ok := config["align"]; ok {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("fuzzy_clock: align must be a string, got %T", v)
		}
		switch s {
		case "center":
			align = AlignCenter
		case "left":
			align = AlignLeft
		case "right":
			align = AlignRight
		default:
			return nil, fmt.Errorf("fuzzy_clock: invalid align %q (must be center, left, or right)", s)
		}
	}

	scale := 1
	if v, ok := config["scale"]; ok {
		n, ok := v.(int)
		if !ok {
			return nil, fmt.Errorf("fuzzy_clock: scale must be an integer, got %T", v)
		}
		if n < 1 {
			return nil, fmt.Errorf("fuzzy_clock: scale must be at least 1, got %d", n)
		}
		scale = n
	}

	now := deps.Now
	if now == nil {
		now = time.Now
	}
	w := New(bounds, now, opts, align, scale)
	if err := w.fitError(); err != nil {
		return nil, err
	}
	return w, nil
}

// Phrase renders t as a natural-language English phrase. It is the pure,
// deterministic core of the widget: the same (t, opts) always yields the same
// string.
//
// It is exported so a widget that cannot use the fuzzy_clock widget itself can
// still borrow its wording. The widget fills its bounds white and draws black
// text; a caller wanting white-on-black, or any other treatment, takes the
// string and does its own drawing. Sharing the function rather than the
// rounding is the point: two clocks on one panel must never disagree at :57.
//
// Minutes are rounded to the nearest five-minute mark and the mark alone
// determines the phrase, so the string never changes mid-mark. The two marks
// flanking the half hour read relative to it — :25 → "about half past", :35 →
// "just after half past" — sidestepping the clumsy "twenty-five past/to". Every
// other mark is precise (including the top of the hour: "five to"/"five past").
func Phrase(t time.Time, opts Options) string {
	hour := t.Hour()
	m := t.Minute()

	// Round to the nearest five-minute mark. r lands in {0,5,...,55,60};
	// 60 rolls into the next hour at the top.
	r := ((m + 2) / 5) * 5
	if r == 60 {
		r = 0
		hour++
	}

	minutes, nextHour := minutesPhrase(r)
	if nextHour {
		hour++
	}
	hourWord := hourPhrase(hour, opts)

	switch {
	case minutes != "":
		return applyStyle(minutes+" "+hourWord, opts.Style)
	case hourWord == "noon" || hourWord == "midnight":
		// "noon"/"midnight" are complete hour references; "noon o'clock"
		// would read wrong, so the o'clock suffix is dropped.
		return applyStyle(hourWord, opts.Style)
	default:
		return applyStyle(hourWord+" o'clock", opts.Style)
	}
}

// minutesPhrase returns the minutes portion of the phrase for a rounded mark r
// (a multiple of 5 in 0..55) and whether the hour reference is the next hour.
// An empty string signals the top of the hour ("o'clock"). The :25 and :35
// marks read relative to the half hour and stay anchored to the current hour.
func minutesPhrase(r int) (phrase string, nextHour bool) {
	switch {
	case r == 0:
		return "", false
	case r == 15:
		return "quarter past", false
	case r == 25:
		return "about half past", false
	case r < 30:
		return numberToWords(r) + " past", false
	case r == 30:
		return "half past", false
	case r == 35:
		return "just after half past", false
	case r == 45:
		return "quarter to", true
	default:
		return numberToWords(60-r) + " to", true
	}
}

// hourPhrase returns the spoken hour word for the given 0..24 hour and options.
func hourPhrase(hour int, opts Options) string {
	hour %= 24
	if opts.Use24Hour {
		return numberToWords(hour)
	}
	h12 := hour % 12
	if opts.NoonMidnight {
		switch hour {
		case 0:
			return "midnight"
		case 12:
			return "noon"
		}
	}
	if h12 == 0 {
		h12 = 12
	}
	return numberToWords(h12)
}

// applyStyle adjusts the casing of an all-lowercase phrase.
func applyStyle(phrase string, s Style) string {
	switch s {
	case StyleTitle:
		words := strings.Split(phrase, " ")
		for i, w := range words {
			words[i] = capitalize(w)
		}
		return strings.Join(words, " ")
	case StyleLower:
		return phrase
	default: // StyleSentence
		return capitalize(phrase)
	}
}

// capitalize upper-cases the first letter of an ASCII word. Callers always
// pass non-empty words: applyStyle works on fixed phrases that are never empty
// and contain no double spaces, so Split never yields an empty token.
func capitalize(w string) string {
	return strings.ToUpper(w[:1]) + w[1:]
}

// numberToWords spells an integer in 0..59 as lowercase English words. It
// covers both the minute words (five, ten, twenty, twenty-five) and the hour
// words (zero..twenty-three). The "quarter" substitution for 15/45 is handled
// by minutesPhrase, not here, so hour 15 in 24-hour mode reads "fifteen".
func numberToWords(n int) string {
	ones := []string{"zero", "one", "two", "three", "four", "five", "six",
		"seven", "eight", "nine", "ten", "eleven", "twelve", "thirteen",
		"fourteen", "fifteen", "sixteen", "seventeen", "eighteen", "nineteen"}
	if n < 20 {
		return ones[n]
	}
	tens := map[int]string{20: "twenty", 30: "thirty", 40: "forty", 50: "fifty"}
	t := (n / 10) * 10
	if n%10 == 0 {
		return tens[t]
	}
	return tens[t] + "-" + ones[n%10]
}
