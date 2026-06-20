package fonts

import (
	_ "embed"
	"fmt"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

//go:embed TerminusTTF-Regular.ttf
var terminusRegularTTF []byte

//go:embed TerminusTTF-Bold.ttf
var terminusBoldTTF []byte

type Weight int

const (
	Regular  Weight = iota
	SemiBold        // maps to Bold (Terminus has no SemiBold)
	Bold
)

var (
	parsedFonts [2]*opentype.Font
	parseOnce   sync.Once
	parseErr    error
)

// fontData is the source of truth for which embedded TTFs the parser
// reads. Tests swap this via SwapDataForTest so the parseErr branch
// of parseFonts is reachable.
var fontData = [2][]byte{terminusRegularTTF, terminusBoldTTF}

func parseFonts() {
	parseOnce.Do(func() {
		for i, d := range fontData {
			f, err := opentype.Parse(d)
			if err != nil {
				parseErr = fmt.Errorf("parse font %d: %w", i, err)
				return
			}
			parsedFonts[i] = f
		}
	})
}

// SwapDataForTest replaces the embedded TTF data the parser reads and
// resets the package's once-state so the next Face call re-parses
// with whatever was supplied. Returns a restore function that puts
// the original data and a fresh (un-parsed) state back. Test-only:
// callers must always defer the restore.
func SwapDataForTest(regular, bold []byte) (restore func()) {
	origData := fontData
	origParsed := parsedFonts
	origErr := parseErr

	fontData = [2][]byte{regular, bold}
	parsedFonts = [2]*opentype.Font{}
	parseErr = nil
	parseOnce = sync.Once{}

	return func() {
		fontData = origData
		parsedFonts = origParsed
		parseErr = origErr
		parseOnce = sync.Once{}
	}
}

func Face(weight Weight, sizePt float64) (font.Face, error) {
	parseFonts()
	if parseErr != nil {
		return nil, parseErr
	}
	idx := 0
	if weight >= SemiBold {
		idx = 1
	}
	// HintingVertical keeps baselines and stems aligned to whole pixels
	// while permitting horizontal anti-aliasing along glyph edges. With
	// every text path now sourcing PaperBlack (Y=0), the AA fringe spans
	// 0..255 and the BW packer's Y<=128 threshold inks every pixel that is
	// at least half covered — so a stem's ~50%-covered centre stays black,
	// keeping glyph shapes intact (see packBW / inkwell-5yh).
	// HintingFull was briefly tried here to suppress AA entirely, but at
	// 10–12 pt sizes it snapped thin features (notably the J's descender
	// hook in Terminus) to zero pixels at some column offsets, so "JUN"
	// rendered as ".UN" in several day-header columns. Vertical hinting
	// + a black text source is the combination that's actually
	// device-durable.
	return opentype.NewFace(parsedFonts[idx], &opentype.FaceOptions{
		Size:    sizePt,
		DPI:     96,
		Hinting: font.HintingVertical,
	})
}
