package weatherview

import (
	"testing"

	"github.com/grantlucas/inkwell/internal/inkwell/weather"
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
		err := DrawIcon(frame, 2, 2, 24, cond)
		if err != nil {
			t.Errorf("DrawIcon(cond=%d): %v", cond, err)
			continue
		}

		hasBlack := false
		for y := range 40 {
			for x := range 40 {
				if frame.ColorIndexAt(x, y) == 1 {
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

func TestDrawIcon_UnknownCondition(t *testing.T) {
	frame := newTestFrame(40, 40)
	err := DrawIcon(frame, 2, 2, 24, weather.Condition(99))
	if err != nil {
		t.Errorf("unexpected error for unknown condition: %v", err)
	}
}

func TestDrawIcon_DifferentSizes(t *testing.T) {
	sizes := []int{12, 16, 24, 32}
	for _, size := range sizes {
		frame := newTestFrame(size+10, size+10)
		err := DrawIcon(frame, 2, 2, size, weather.Clear)
		if err != nil {
			t.Errorf("DrawIcon(size=%d): %v", size, err)
		}
	}
}

func TestIconFace(t *testing.T) {
	f, err := iconFace(24)
	if err != nil {
		t.Fatalf("iconFace: %v", err)
	}
	f.Close()
}

func TestDrawIcon_BadFontData(t *testing.T) {
	orig := fontData
	fontData = []byte("not a font")
	defer func() { fontData = orig }()

	frame := newTestFrame(40, 40)
	err := DrawIcon(frame, 2, 2, 24, weather.Clear)
	if err == nil {
		t.Fatal("expected error with bad font data")
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

func TestIconFace_ZeroSize(t *testing.T) {
	f, err := iconFace(0)
	if err != nil {
		return
	}
	f.Close()
}

func TestDrawIcon_GlyphNotFound(t *testing.T) {
	orig := conditionGlyphs[weather.Clear]
	conditionGlyphs[weather.Clear] = 0x0001
	defer func() { conditionGlyphs[weather.Clear] = orig }()

	frame := newTestFrame(40, 40)
	err := DrawIcon(frame, 2, 2, 24, weather.Clear)
	if err == nil {
		t.Fatal("expected glyph not found error")
	}
}
