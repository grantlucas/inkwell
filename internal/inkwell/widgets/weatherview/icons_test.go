package weatherview

import (
	"bytes"
	"errors"
	"log"
	"os"
	"strings"
	"testing"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"

	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

func TestDrawIcon_AllConditions(t *testing.T) {
	conditions := []weather.Condition{
		weather.Clear,
		weather.PartlyCloudy,
		weather.Cloudy,
		weather.Rain,
		weather.Snow,
		weather.Thunderstorm,
		weather.Fog,
		weather.Drizzle,
	}

	for _, cond := range conditions {
		frame := newTestFrame(40, 40)
		DrawIcon(frame, 2, 2, 24, cond)

		// Look up the semantic black index instead of hardcoding
		// `1`; the palette layout is allowed to shift and tests
		// shouldn't pin to its current numeric address.
		hasBlack := false
		for y := range 40 {
			for x := range 40 {
				if frame.ColorIndexAt(x, y) == widget.PaperBlack {
					hasBlack = true
					break
				}
			}
			if hasBlack {
				break
			}
		}
		if !hasBlack {
			t.Errorf("DrawIcon(cond=%d) drew no black pixels", cond)
		}
	}
}

// An unknown condition falls back to the clear-sky glyph rather than
// leaving the cell empty.
func TestDrawIcon_UnknownCondition(t *testing.T) {
	unknown := newTestFrame(40, 40)
	DrawIcon(unknown, 2, 2, 24, weather.Condition(99))
	clear := newTestFrame(40, 40)
	DrawIcon(clear, 2, 2, 24, weather.Clear)

	if !bytes.Equal(unknown.Pix, clear.Pix) {
		t.Error("unknown condition did not draw the clear-sky glyph")
	}
}

func TestDrawIcon_DifferentSizes(t *testing.T) {
	sizes := []int{12, 16, 24, 32}
	for _, size := range sizes {
		frame := newTestFrame(size+10, size+10)
		DrawIcon(frame, 2, 2, size, weather.Clear)
		if !chartHasInk(frame) {
			t.Errorf("DrawIcon(size=%d) drew nothing", size)
		}
	}
}

// captureLog redirects the standard logger for the rest of the test and
// returns what it wrote.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return &buf
}

// A glyph that will not draw is logged, not returned: the rest of the
// cell — the temperatures and the chart — is still worth drawing, so no
// caller has anything to do with the error but log it.
func TestDrawIcon_FailureIsLoggedAndDrawsNothing(t *testing.T) {
	cases := []struct {
		label     string
		breakIcon func(t *testing.T)
	}{
		{"bad font data", func(t *testing.T) {
			orig := fontData
			fontData = []byte("not a font")
			t.Cleanup(func() { fontData = orig })
		}},
		{"glyph not found", func(t *testing.T) {
			orig := conditionGlyphs[weather.Clear]
			conditionGlyphs[weather.Clear] = 0x0001
			t.Cleanup(func() { conditionGlyphs[weather.Clear] = orig })
		}},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			tc.breakIcon(t)
			logged := captureLog(t)

			frame := newTestFrame(40, 40)
			DrawIcon(frame, 2, 2, 24, weather.Clear)

			if chartHasInk(frame) {
				t.Error("drew ink for a glyph that failed")
			}
			if !strings.Contains(logged.String(), "weatherview: draw icon") {
				t.Errorf("log = %q, want a weatherview draw icon message", logged.String())
			}
		})
	}
}

func TestIconFace(t *testing.T) {
	f, err := iconFace(24)
	if err != nil {
		t.Fatalf("iconFace: %v", err)
	}
	if cerr := f.Close(); cerr != nil {
		t.Errorf("close face: %v", cerr)
	}
}

func TestIconFace_BadFontData(t *testing.T) {
	orig := fontData
	fontData = []byte("not a font")
	defer func() { fontData = orig }()

	_, err := iconFace(24)
	if err == nil {
		t.Fatal("expected error")
	}
}

// iconFace clamps size to a minimum of 1, so a zero-size request must
// succeed and return a usable face. Accepting either branch (as the
// previous test did) meant the assertion didn't actually pin down
// behavior; a future regression that started returning an error here
// would have slipped past silently.
func TestIconFace_ZeroSize(t *testing.T) {
	f, err := iconFace(0)
	if err != nil {
		t.Fatalf("iconFace(0): unexpected error %v", err)
	}
	if f == nil {
		t.Fatal("iconFace(0): face is nil")
	}
	if cerr := f.Close(); cerr != nil {
		t.Errorf("close face: %v", cerr)
	}
}

// The opentype.NewFace error path is structurally unreachable with a
// valid embedded font + bounded options, so iconFace routes through
// the newOpenTypeFace indirection that tests override. Swap in a
// stub that always errors so the wrap-and-return branch is covered.
func TestIconFace_OpenTypeNewFaceError(t *testing.T) {
	orig := newOpenTypeFace
	defer func() { newOpenTypeFace = orig }()
	newOpenTypeFace = func(*opentype.Font, *opentype.FaceOptions) (font.Face, error) {
		return nil, errors.New("injected NewFace failure")
	}

	_, err := iconFace(24)
	if err == nil {
		t.Fatal("expected error from injected NewFace stub")
	}
}
