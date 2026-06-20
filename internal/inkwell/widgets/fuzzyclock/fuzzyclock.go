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
// configured options (see fuzzyTime): the same minute always produces the same
// string, so the widget never surprises the refresh queue with an unexpected
// change.
package fuzzyclock

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"strings"
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"

	"github.com/grantlucas/inkwell/internal/inkwell/fonts"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
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

// style controls the letter casing of the rendered phrase.
type style int

const (
	styleSentence style = iota // "About half past eight" (default)
	styleTitle                 // "About Half Past Eight"
	styleLower                 // "about half past eight"
)

// Align controls horizontal alignment of the phrase within the widget bounds.
type Align int

const (
	AlignCenter Align = iota
	AlignLeft
	AlignRight
)

// options bundles the rendering knobs for fuzzyTime and placement. The zero
// value aligns center, which preserves the widget's original behavior.
type options struct {
	style        style
	noonMidnight bool  // substitute "noon"/"midnight" for "twelve" (12-hour only)
	use24Hour    bool  // spell the hour as 0..23 instead of 1..12
	align        Align // horizontal alignment within bounds
}

// Widget renders the current time as a natural-language English phrase.
type Widget struct {
	bounds image.Rectangle
	now    func() time.Time
	opts   options
}

// New creates a fuzzy clock Widget. A nil now falls back to time.Now so the
// widget renders something reasonable when callers wire it up without an
// explicit clock.
func New(bounds image.Rectangle, now func() time.Time, opts options) *Widget {
	if now == nil {
		now = time.Now
	}
	return &Widget{bounds: bounds, now: now, opts: opts}
}

// Bounds returns the rectangle this widget occupies on the display.
func (w *Widget) Bounds() image.Rectangle { return w.bounds }

// Render draws the fuzzy time within the bounds using black text on a white
// background, aligned per opts.align (center by default). Text sources
// PaperBlack so the anti-aliased glyph fringe straddles the BW threshold
// cleanly (see fonts.Face / project rendering rules). Left/right alignment
// insets the text 4px from the matching edge, matching the clock widget.
func (w *Widget) Render(frame *image.Paletted) error {
	draw.Draw(frame, w.bounds, image.NewUniform(color.White), image.Point{}, draw.Src)

	text := fuzzyTime(w.now(), w.opts)
	textW := font.MeasureString(fuzzyFace, text).Ceil()
	metrics := fuzzyFace.Metrics()
	textH := (metrics.Ascent + metrics.Descent).Ceil()

	var x int
	switch w.opts.align {
	case AlignLeft:
		x = w.bounds.Min.X + 4
	case AlignRight:
		x = w.bounds.Max.X - textW - 4
	default:
		x = w.bounds.Min.X + (w.bounds.Dx()-textW)/2
	}
	y := w.bounds.Min.Y + (w.bounds.Dy()-textH)/2 + metrics.Ascent.Ceil()

	d := &font.Drawer{
		Dst:  frame,
		Src:  image.NewUniform(color.Black),
		Face: fuzzyFace,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(text)

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
func Factory(bounds image.Rectangle, config map[string]any, deps widget.Deps) (widget.Widget, error) {
	opts := options{style: styleSentence, noonMidnight: true}

	if v, ok := config["style"]; ok {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("fuzzy_clock: style must be a string, got %T", v)
		}
		switch s {
		case "sentence":
			opts.style = styleSentence
		case "title":
			opts.style = styleTitle
		case "lower":
			opts.style = styleLower
		default:
			return nil, fmt.Errorf("fuzzy_clock: invalid style %q (must be sentence, title, or lower)", s)
		}
	}

	if v, ok := config["use_words_for_noon_and_midnight"]; ok {
		b, ok := v.(bool)
		if !ok {
			return nil, fmt.Errorf("fuzzy_clock: use_words_for_noon_and_midnight must be a bool, got %T", v)
		}
		opts.noonMidnight = b
	}

	if v, ok := config["use_24_hour"]; ok {
		b, ok := v.(bool)
		if !ok {
			return nil, fmt.Errorf("fuzzy_clock: use_24_hour must be a bool, got %T", v)
		}
		opts.use24Hour = b
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
			opts.align = AlignCenter
		case "left":
			opts.align = AlignLeft
		case "right":
			opts.align = AlignRight
		default:
			return nil, fmt.Errorf("fuzzy_clock: invalid align %q (must be center, left, or right)", s)
		}
	}

	now := deps.Now
	if now == nil {
		now = time.Now
	}
	return New(bounds, now, opts), nil
}

// fuzzyTime renders t as a natural-language English phrase. It is the pure,
// deterministic core of the widget: the same (t, opts) always yields the same
// string.
//
// Minutes are rounded to the nearest five-minute mark and the mark alone
// determines the phrase, so the string never changes mid-mark. The two marks
// flanking the half hour read relative to it — :25 → "about half past", :35 →
// "just after half past" — sidestepping the clumsy "twenty-five past/to". Every
// other mark is precise (including the top of the hour: "five to"/"five past").
func fuzzyTime(t time.Time, opts options) string {
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
		return applyStyle(minutes+" "+hourWord, opts.style)
	case hourWord == "noon" || hourWord == "midnight":
		// "noon"/"midnight" are complete hour references; "noon o'clock"
		// would read wrong, so the o'clock suffix is dropped.
		return applyStyle(hourWord, opts.style)
	default:
		return applyStyle(hourWord+" o'clock", opts.style)
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
func hourPhrase(hour int, opts options) string {
	hour %= 24
	if opts.use24Hour {
		return numberToWords(hour)
	}
	h12 := hour % 12
	if opts.noonMidnight {
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
func applyStyle(phrase string, s style) string {
	switch s {
	case styleTitle:
		words := strings.Split(phrase, " ")
		for i, w := range words {
			words[i] = capitalize(w)
		}
		return strings.Join(words, " ")
	case styleLower:
		return phrase
	default: // styleSentence
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
