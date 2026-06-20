package fuzzyclock

import (
	"image"
	"strings"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/fonts"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

func at(h, m int) time.Time {
	return time.Date(2026, 6, 19, h, m, 0, 0, time.UTC)
}

func fixedClock(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

// TestFuzzyTime_FullMinuteRange walks every minute of the 8 o'clock hour so the
// nearest-five-minute rounding, the o'clock/half/quarter special phrases, the
// half-past-relative :25/:35 marks ("about half past"/"just after half past"),
// the past→to flip, and the hour rollover at :58/:59 are all exercised in one
// table. Every minute reads as its nearest five-minute mark; there is no
// per-minute "about" softener.
func TestFuzzyTime_FullMinuteRange(t *testing.T) {
	opts := options{style: styleSentence, noonMidnight: true}
	cases := []struct {
		minute int
		want   string
	}{
		{0, "Eight o'clock"},
		{1, "Eight o'clock"},
		{2, "Eight o'clock"},
		{3, "Five past eight"},
		{4, "Five past eight"},
		{5, "Five past eight"},
		{6, "Five past eight"},
		{7, "Five past eight"},
		{8, "Ten past eight"},
		{9, "Ten past eight"},
		{10, "Ten past eight"},
		{11, "Ten past eight"},
		{12, "Ten past eight"},
		{13, "Quarter past eight"},
		{14, "Quarter past eight"},
		{15, "Quarter past eight"},
		{16, "Quarter past eight"},
		{17, "Quarter past eight"},
		{18, "Twenty past eight"},
		{19, "Twenty past eight"},
		{20, "Twenty past eight"},
		{21, "Twenty past eight"},
		{22, "Twenty past eight"},
		{23, "About half past eight"},
		{24, "About half past eight"},
		{25, "About half past eight"},
		{26, "About half past eight"},
		{27, "About half past eight"},
		{28, "Half past eight"},
		{29, "Half past eight"},
		{30, "Half past eight"},
		{31, "Half past eight"},
		{32, "Half past eight"},
		{33, "Just after half past eight"},
		{34, "Just after half past eight"},
		{35, "Just after half past eight"},
		{36, "Just after half past eight"},
		{37, "Just after half past eight"},
		{38, "Twenty to nine"},
		{39, "Twenty to nine"},
		{40, "Twenty to nine"},
		{41, "Twenty to nine"},
		{42, "Twenty to nine"},
		{43, "Quarter to nine"},
		{44, "Quarter to nine"},
		{45, "Quarter to nine"},
		{46, "Quarter to nine"},
		{47, "Quarter to nine"},
		{48, "Ten to nine"},
		{49, "Ten to nine"},
		{50, "Ten to nine"},
		{51, "Ten to nine"},
		{52, "Ten to nine"},
		{53, "Five to nine"},
		{54, "Five to nine"},
		{55, "Five to nine"},
		{56, "Five to nine"},
		{57, "Five to nine"},
		{58, "Nine o'clock"},
		{59, "Nine o'clock"},
	}
	for _, tc := range cases {
		if got := fuzzyTime(at(8, tc.minute), opts); got != tc.want {
			t.Errorf("fuzzyTime(8:%02d) = %q, want %q", tc.minute, got, tc.want)
		}
	}
}

// TestFuzzyTime_Determinism guards acceptance criterion (2): a fixed instant
// always renders the same string.
func TestFuzzyTime_Determinism(t *testing.T) {
	opts := options{style: styleSentence, noonMidnight: true}
	first := fuzzyTime(at(8, 9), opts)
	for range 100 {
		if got := fuzzyTime(at(8, 9), opts); got != first {
			t.Fatalf("non-deterministic: got %q, first %q", got, first)
		}
	}
}

// TestFuzzyTime_Options covers noon/midnight (on and off), 24-hour mode, the
// midnight rollover, and the casing styles.
func TestFuzzyTime_Options(t *testing.T) {
	cases := []struct {
		label string
		when  time.Time
		opts  options
		want  string
	}{
		{"noon on", at(12, 0), options{style: styleSentence, noonMidnight: true}, "Noon"},
		{"midnight on", at(0, 0), options{style: styleSentence, noonMidnight: true}, "Midnight"},
		{"about half past noon", at(12, 25), options{style: styleSentence, noonMidnight: true}, "About half past noon"},
		{"half past noon", at(12, 30), options{style: styleSentence, noonMidnight: true}, "Half past noon"},
		{"just after half past noon", at(12, 35), options{style: styleSentence, noonMidnight: true}, "Just after half past noon"},
		{"noon rolls from 11:58", at(11, 58), options{style: styleSentence, noonMidnight: true}, "Noon"},
		{"midnight rolls from 23:58", at(23, 58), options{style: styleSentence, noonMidnight: true}, "Midnight"},
		{"noon off", at(12, 0), options{style: styleSentence, noonMidnight: false}, "Twelve o'clock"},
		{"midnight off", at(0, 0), options{style: styleSentence, noonMidnight: false}, "Twelve o'clock"},
		{"12-hour pm maps down", at(20, 30), options{style: styleSentence, noonMidnight: true}, "Half past eight"},
		{"24-hour evening", at(20, 30), options{style: styleSentence, use24Hour: true}, "Half past twenty"},
		{"24-hour ignores noon word", at(12, 0), options{style: styleSentence, noonMidnight: true, use24Hour: true}, "Twelve o'clock"},
		{"24-hour midnight is zero", at(0, 0), options{style: styleSentence, use24Hour: true}, "Zero o'clock"},
		{"24-hour rolls to next hour", at(8, 40), options{style: styleSentence, use24Hour: true}, "Twenty to nine"},
		{"title style", at(8, 30), options{style: styleTitle, noonMidnight: true}, "Half Past Eight"},
		{"lower style", at(8, 30), options{style: styleLower, noonMidnight: true}, "half past eight"},
		{"title style hyphenated", at(21, 30), options{style: styleTitle, use24Hour: true}, "Half Past Twenty-one"},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			if got := fuzzyTime(tc.when, tc.opts); got != tc.want {
				t.Errorf("fuzzyTime = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestWidget_Bounds(t *testing.T) {
	bounds := image.Rect(0, 0, 800, 50)
	w := New(bounds, fixedClock(at(8, 30)), options{})
	if got := w.Bounds(); got != bounds {
		t.Errorf("Bounds() = %v, want %v", got, bounds)
	}
}

func TestWidget_Render(t *testing.T) {
	bounds := image.Rect(0, 0, 800, 50)
	w := New(bounds, fixedClock(at(8, 30)), options{style: styleSentence, noonMidnight: true})

	frame := image.NewPaletted(bounds, widget.PaperPalette)
	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}

	hasBlack := false
	for _, px := range frame.Pix {
		if px != 0 {
			hasBlack = true
			break
		}
	}
	if !hasBlack {
		t.Error("rendered blank frame")
	}
}

func TestNew_NilNow(t *testing.T) {
	w := New(image.Rect(0, 0, 800, 50), nil, options{})
	frame := image.NewPaletted(image.Rect(0, 0, 800, 50), widget.PaperPalette)
	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}
}

func TestFactory(t *testing.T) {
	deps := widget.Deps{Now: fixedClock(at(8, 30))}
	bounds := image.Rect(0, 0, 800, 50)

	cases := []struct {
		label   string
		config  map[string]any
		wantErr bool
		// want is the rendered string when no error is expected; checked via
		// fuzzyTime on the constructed widget's options through a render proxy.
		want string
	}{
		{label: "defaults", config: nil, want: "Half past eight"},
		{label: "style title", config: map[string]any{"style": "title"}, want: "Half Past Eight"},
		{label: "style lower", config: map[string]any{"style": "lower"}, want: "half past eight"},
		{label: "style sentence", config: map[string]any{"style": "sentence"}, want: "Half past eight"},
		{label: "noon/midnight off", config: map[string]any{"use_words_for_noon_and_midnight": false}, want: "Half past eight"},
		{label: "24 hour", config: map[string]any{"use_24_hour": true}, want: "Half past eight"},
		{label: "language en", config: map[string]any{"language": "en"}, want: "Half past eight"},
		{label: "align center", config: map[string]any{"align": "center"}, want: "Half past eight"},
		{label: "align left", config: map[string]any{"align": "left"}, want: "Half past eight"},
		{label: "align right", config: map[string]any{"align": "right"}, want: "Half past eight"},
		{label: "invalid style value", config: map[string]any{"style": "shouting"}, wantErr: true},
		{label: "style wrong type", config: map[string]any{"style": 123}, wantErr: true},
		{label: "noon/midnight wrong type", config: map[string]any{"use_words_for_noon_and_midnight": "yes"}, wantErr: true},
		{label: "24 hour wrong type", config: map[string]any{"use_24_hour": "yes"}, wantErr: true},
		{label: "language wrong type", config: map[string]any{"language": 5}, wantErr: true},
		{label: "language unsupported", config: map[string]any{"language": "fr"}, wantErr: true},
		{label: "align invalid value", config: map[string]any{"align": "middle"}, wantErr: true},
		{label: "align wrong type", config: map[string]any{"align": 42}, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			w, err := Factory(bounds, tc.config, deps)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Factory: %v", err)
			}
			fw, ok := w.(*Widget)
			if !ok {
				t.Fatalf("Factory returned %T, want *Widget", w)
			}
			if got := fuzzyTime(fw.now(), fw.opts); got != tc.want {
				t.Errorf("rendered %q, want %q", got, tc.want)
			}
		})
	}
}

// TestFactory_Align renders the same phrase under left/right alignment and
// asserts the inked pixels cluster in the matching half of the bounds. The
// bounds are wide (800px) relative to the phrase so a left-aligned phrase lands
// entirely in the left half and a right-aligned one entirely in the right half.
func TestFactory_Align(t *testing.T) {
	deps := widget.Deps{Now: fixedClock(at(8, 30))}
	bounds := image.Rect(0, 0, 800, 50)
	mid := bounds.Dx() / 2

	countHalves := func(frame *image.Paletted) (left, right int) {
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				if frame.ColorIndexAt(x, y) == 1 {
					if x < mid {
						left++
					} else {
						right++
					}
				}
			}
		}
		return left, right
	}

	cases := []struct {
		label     string
		align     string
		wantLeft  bool // expect inked pixels in the left half
		wantRight bool // expect inked pixels in the right half
	}{
		{label: "left", align: "left", wantLeft: true, wantRight: false},
		{label: "right", align: "right", wantLeft: false, wantRight: true},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			w, err := Factory(bounds, map[string]any{"align": tc.align}, deps)
			if err != nil {
				t.Fatalf("Factory: %v", err)
			}
			frame := image.NewPaletted(bounds, widget.PaperPalette)
			if err := w.Render(frame); err != nil {
				t.Fatalf("Render: %v", err)
			}
			left, right := countHalves(frame)
			if tc.wantLeft && left == 0 {
				t.Errorf("%s-aligned: no inked pixels in left half", tc.align)
			}
			if !tc.wantLeft && left != 0 {
				t.Errorf("%s-aligned: %d inked pixels in left half, want 0", tc.align, left)
			}
			if tc.wantRight && right == 0 {
				t.Errorf("%s-aligned: no inked pixels in right half", tc.align)
			}
			if !tc.wantRight && right != 0 {
				t.Errorf("%s-aligned: %d inked pixels in right half, want 0", tc.align, right)
			}
		})
	}
}

func TestFactory_NilNow(t *testing.T) {
	w, err := Factory(image.Rect(0, 0, 800, 50), nil, widget.Deps{})
	if err != nil {
		t.Fatalf("Factory: %v", err)
	}
	frame := image.NewPaletted(image.Rect(0, 0, 800, 50), widget.PaperPalette)
	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}
}

// mustLoadFuzzyFace runs at package init with valid embedded fonts. Pin its
// panic branch by swapping in bad TTF data and re-invoking it directly.
func TestMustLoadFuzzyFace_PanicsOnFontError(t *testing.T) {
	restore := fonts.SwapDataForTest([]byte("bad"), []byte("bad"))
	defer restore()

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic from mustLoadFuzzyFace")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "fuzzy_clock: load font") {
			t.Errorf("panic = %v, want a string mentioning 'fuzzy_clock: load font'", r)
		}
	}()
	_ = mustLoadFuzzyFace()
}
