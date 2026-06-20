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

// TestFuzzyTime_FullMinuteRange walks every minute of the 8 o'clock hour so
// each five-minute mark, both qualifier directions, the o'clock/half/quarter
// special phrases, the past→to flip at 30, and the hour rollover at :58/:59 are
// all exercised in one table.
func TestFuzzyTime_FullMinuteRange(t *testing.T) {
	opts := options{style: styleSentence, noonMidnight: true}
	cases := []struct {
		minute int
		want   string
	}{
		{0, "Eight o'clock"},
		{1, "About eight o'clock"},
		{2, "About eight o'clock"},
		{3, "About five past eight"},
		{4, "About five past eight"},
		{5, "Five past eight"},
		{6, "About five past eight"},
		{7, "About five past eight"},
		{8, "About ten past eight"},
		{9, "About ten past eight"},
		{10, "Ten past eight"},
		{11, "About ten past eight"},
		{12, "About ten past eight"},
		{13, "About quarter past eight"},
		{14, "About quarter past eight"},
		{15, "Quarter past eight"},
		{16, "About quarter past eight"},
		{17, "About quarter past eight"},
		{18, "About twenty past eight"},
		{19, "About twenty past eight"},
		{20, "Twenty past eight"},
		{21, "About twenty past eight"},
		{22, "About twenty past eight"},
		{23, "About twenty-five past eight"},
		{24, "About twenty-five past eight"},
		{25, "Twenty-five past eight"},
		{26, "About twenty-five past eight"},
		{27, "About twenty-five past eight"},
		{28, "About half past eight"},
		{29, "About half past eight"},
		{30, "Half past eight"},
		{31, "About half past eight"},
		{32, "About half past eight"},
		{33, "About twenty-five to nine"},
		{34, "About twenty-five to nine"},
		{35, "Twenty-five to nine"},
		{36, "About twenty-five to nine"},
		{37, "About twenty-five to nine"},
		{38, "About twenty to nine"},
		{39, "About twenty to nine"},
		{40, "Twenty to nine"},
		{41, "About twenty to nine"},
		{42, "About twenty to nine"},
		{43, "About quarter to nine"},
		{44, "About quarter to nine"},
		{45, "Quarter to nine"},
		{46, "About quarter to nine"},
		{47, "About quarter to nine"},
		{48, "About ten to nine"},
		{49, "About ten to nine"},
		{50, "Ten to nine"},
		{51, "About ten to nine"},
		{52, "About ten to nine"},
		{53, "About five to nine"},
		{54, "About five to nine"},
		{55, "Five to nine"},
		{56, "About five to nine"},
		{57, "About five to nine"},
		{58, "About nine o'clock"},
		{59, "About nine o'clock"},
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
		{"about midnight", at(0, 1), options{style: styleSentence, noonMidnight: true}, "About midnight"},
		{"half past noon", at(12, 30), options{style: styleSentence, noonMidnight: true}, "Half past noon"},
		{"about noon rolls from 11:58", at(11, 58), options{style: styleSentence, noonMidnight: true}, "About noon"},
		{"about midnight rolls from 23:58", at(23, 58), options{style: styleSentence, noonMidnight: true}, "About midnight"},
		{"noon off", at(12, 0), options{style: styleSentence, noonMidnight: false}, "Twelve o'clock"},
		{"midnight off", at(0, 0), options{style: styleSentence, noonMidnight: false}, "Twelve o'clock"},
		{"12-hour pm maps down", at(20, 30), options{style: styleSentence, noonMidnight: true}, "Half past eight"},
		{"24-hour evening", at(20, 30), options{style: styleSentence, use24Hour: true}, "Half past twenty"},
		{"24-hour ignores noon word", at(12, 0), options{style: styleSentence, noonMidnight: true, use24Hour: true}, "Twelve o'clock"},
		{"24-hour midnight is zero", at(0, 0), options{style: styleSentence, use24Hour: true}, "Zero o'clock"},
		{"24-hour rolls to next hour", at(8, 40), options{style: styleSentence, use24Hour: true}, "Twenty to nine"},
		{"title style", at(8, 30), options{style: styleTitle, noonMidnight: true}, "Half Past Eight"},
		{"lower style", at(8, 30), options{style: styleLower, noonMidnight: true}, "half past eight"},
		{"title style hyphenated", at(8, 25), options{style: styleTitle, noonMidnight: true}, "Twenty-five Past Eight"},
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
		{label: "invalid style value", config: map[string]any{"style": "shouting"}, wantErr: true},
		{label: "style wrong type", config: map[string]any{"style": 123}, wantErr: true},
		{label: "noon/midnight wrong type", config: map[string]any{"use_words_for_noon_and_midnight": "yes"}, wantErr: true},
		{label: "24 hour wrong type", config: map[string]any{"use_24_hour": "yes"}, wantErr: true},
		{label: "language wrong type", config: map[string]any{"language": 5}, wantErr: true},
		{label: "language unsupported", config: map[string]any{"language": "fr"}, wantErr: true},
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
