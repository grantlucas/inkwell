package fonts

import (
	_ "embed"
	"fmt"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

//go:embed IBMPlexMono-Regular.ttf
var plexRegularTTF []byte

//go:embed IBMPlexMono-SemiBold.ttf
var plexSemiBoldTTF []byte

//go:embed IBMPlexMono-Bold.ttf
var plexBoldTTF []byte

type Weight int

const (
	Regular  Weight = iota
	SemiBold
	Bold
)

var (
	parsedFonts [3]*opentype.Font
	parseOnce   sync.Once
	parseErr    error
)

func parseFonts() {
	parseOnce.Do(func() {
		data := [3][]byte{plexRegularTTF, plexSemiBoldTTF, plexBoldTTF}
		for i, d := range data {
			f, err := opentype.Parse(d)
			if err != nil {
				parseErr = fmt.Errorf("parse font %d: %w", i, err)
				return
			}
			parsedFonts[i] = f
		}
	})
}

func Face(weight Weight, sizePx float64) (font.Face, error) {
	parseFonts()
	if parseErr != nil {
		return nil, parseErr
	}
	return opentype.NewFace(parsedFonts[weight], &opentype.FaceOptions{
		Size:    sizePx,
		DPI:     72,
		Hinting: font.HintingFull,
	})
}
