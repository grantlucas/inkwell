package fonts

import (
	"bufio"
	"bytes"
	"fmt"
	"image"
	"image/color"
	"strconv"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// BDFFont is a parsed BDF (Bitmap Distribution Format) font. It holds one
// pre-rendered 1-bit mask per glyph, so rendering deposits pure black/white
// pixels with no anti-aliasing — exactly what a 1-bit e-paper panel wants.
type BDFFont struct {
	ascent  int // pixels above the baseline
	descent int // pixels below the baseline
	glyphs  map[rune]*bdfGlyph
}

// bdfGlyph is one parsed character: its advance width plus a 1-bit mask and
// the mask's placement relative to the pen origin (baseline-left).
type bdfGlyph struct {
	advance int          // DWIDTH x — pen advance in pixels
	w, h    int          // BBX width/height in pixels
	xoff    int          // BBX x offset from origin
	yoff    int          // BBX y offset from origin (baseline) to bitmap bottom
	mask    *image.Alpha // w x h, 0x00 or 0xFF per pixel
}

// ParseBDF parses a BDF font. It reads the global font ascent/descent (from
// the FONT_ASCENT / FONT_DESCENT properties, falling back to the
// FONTBOUNDINGBOX) and every glyph's DWIDTH, BBX, and BITMAP rows.
func ParseBDF(data []byte) (*BDFFont, error) {
	f := &BDFFont{glyphs: map[rune]*bdfGlyph{}}
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)

	var bbH, bbYoff int
	haveAscent, haveDescent := false, false
	sawChar := false

	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case strings.HasPrefix(line, "FONTBOUNDINGBOX"):
			fields := strings.Fields(line)
			if len(fields) >= 5 {
				bbH = atoi(fields[2])
				bbYoff = atoi(fields[4])
			}
		case strings.HasPrefix(line, "FONT_ASCENT"):
			f.ascent = atoi(lastField(line))
			haveAscent = true
		case strings.HasPrefix(line, "FONT_DESCENT"):
			f.descent = atoi(lastField(line))
			haveDescent = true
		case strings.HasPrefix(line, "STARTCHAR"):
			sawChar = true
			g, r, err := parseGlyph(sc)
			if err != nil {
				return nil, err
			}
			if r >= 0 {
				f.glyphs[r] = g
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read bdf: %w", err)
	}
	if !sawChar {
		return nil, fmt.Errorf("bdf: no glyphs found")
	}
	if !haveAscent {
		f.ascent = bbH + bbYoff
	}
	if !haveDescent {
		f.descent = -bbYoff
	}
	return f, nil
}

// parseGlyph consumes the lines of a single STARTCHAR..ENDCHAR block (the
// scanner is positioned just after the STARTCHAR line). It returns the glyph
// and its rune, or rune -1 for an unencoded glyph that should be skipped.
func parseGlyph(sc *bufio.Scanner) (*bdfGlyph, rune, error) {
	g := &bdfGlyph{}
	r := rune(-1)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case strings.HasPrefix(line, "ENCODING"):
			fields := strings.Fields(line)
			r = rune(atoi(fields[1]))
			if r < 0 && len(fields) >= 3 { // "ENCODING -1 <alt>"
				r = rune(atoi(fields[2]))
			}
		case strings.HasPrefix(line, "DWIDTH"):
			g.advance = atoi(strings.Fields(line)[1])
		case strings.HasPrefix(line, "BBX"):
			fields := strings.Fields(line)
			g.w, g.h = atoi(fields[1]), atoi(fields[2])
			g.xoff, g.yoff = atoi(fields[3]), atoi(fields[4])
		case line == "BITMAP":
			if err := readBitmap(sc, g); err != nil {
				return nil, r, err
			}
		case line == "ENDCHAR":
			return g, r, nil
		}
	}
	return nil, r, fmt.Errorf("bdf: glyph missing ENDCHAR")
}

// readBitmap reads g.h rows of hex following a BITMAP line into a 1-bit mask.
// Each row is ceil(w/8) bytes, packed MSB-first; bit i sets column i.
func readBitmap(sc *bufio.Scanner, g *bdfGlyph) error {
	g.mask = image.NewAlpha(image.Rect(0, 0, g.w, g.h))
	for y := 0; y < g.h; y++ {
		if !sc.Scan() {
			return fmt.Errorf("bdf: bitmap truncated")
		}
		row := strings.TrimSpace(sc.Text())
		for x := 0; x < g.w; x++ {
			nibble := x / 4
			if nibble >= len(row) {
				continue
			}
			v, err := strconv.ParseUint(string(row[nibble]), 16, 8)
			if err != nil {
				return fmt.Errorf("bdf: bad bitmap hex %q: %w", row, err)
			}
			if v&(1<<uint(3-(x%4))) != 0 {
				g.mask.SetAlpha(x, y, color.Alpha{A: 0xFF})
			}
		}
	}
	return nil
}

// NewFace returns a font.Face backed by the parsed glyphs.
func (f *BDFFont) NewFace() font.Face { return &bdfFace{f: f} }

type bdfFace struct{ f *BDFFont }

func (face *bdfFace) Close() error { return nil }

func (face *bdfFace) Metrics() font.Metrics {
	a, d := fixed.I(face.f.ascent), fixed.I(face.f.descent)
	return font.Metrics{
		Height:     a + d,
		Ascent:     a,
		Descent:    d,
		XHeight:    a,
		CapHeight:  a,
		CaretSlope: image.Point{X: 0, Y: 1},
	}
}

func (face *bdfFace) Kern(r0, r1 rune) fixed.Int26_6 { return 0 }

func (face *bdfFace) glyph(r rune) *bdfGlyph {
	if g, ok := face.f.glyphs[r]; ok {
		return g
	}
	return nil
}

func (face *bdfFace) GlyphAdvance(r rune) (fixed.Int26_6, bool) {
	g := face.glyph(r)
	if g == nil {
		return 0, false
	}
	return fixed.I(g.advance), true
}

func (face *bdfFace) GlyphBounds(r rune) (fixed.Rectangle26_6, fixed.Int26_6, bool) {
	g := face.glyph(r)
	if g == nil {
		return fixed.Rectangle26_6{}, 0, false
	}
	b := fixed.Rectangle26_6{
		Min: fixed.Point26_6{X: fixed.I(g.xoff), Y: fixed.I(-(g.yoff + g.h))},
		Max: fixed.Point26_6{X: fixed.I(g.xoff + g.w), Y: fixed.I(-g.yoff)},
	}
	return b, fixed.I(g.advance), true
}

func (face *bdfFace) Glyph(dot fixed.Point26_6, r rune) (
	dr image.Rectangle, mask image.Image, maskp image.Point, advance fixed.Int26_6, ok bool) {
	g := face.glyph(r)
	if g == nil {
		return image.Rectangle{}, nil, image.Point{}, 0, false
	}
	px, py := dot.X.Floor(), dot.Y.Floor()
	dr = image.Rect(px+g.xoff, py-g.yoff-g.h, px+g.xoff+g.w, py-g.yoff)
	return dr, g.mask, image.Point{}, fixed.I(g.advance), true
}

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

func lastField(line string) string {
	fields := strings.Fields(line)
	return fields[len(fields)-1]
}
