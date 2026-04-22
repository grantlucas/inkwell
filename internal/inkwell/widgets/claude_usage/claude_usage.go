package claude_usage

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"

	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

// Compile-time interface check.
var _ widget.Widget = (*Widget)(nil)

// Widget renders Claude API usage as two progress bars with labels.
type Widget struct {
	bounds image.Rectangle
	source widget.UsageSource
	now    func() time.Time
}

// New creates a claude_usage Widget.
func New(bounds image.Rectangle, source widget.UsageSource, now func() time.Time) *Widget {
	return &Widget{bounds: bounds, source: source, now: now}
}

// Factory creates a claude_usage Widget from config and dependencies.
func Factory(bounds image.Rectangle, _ map[string]any, deps widget.Deps) (widget.Widget, error) {
	if deps.UsageSource == nil {
		return nil, fmt.Errorf("claude_usage: UsageSource dependency is required")
	}
	now := deps.Now
	if now == nil {
		now = time.Now
	}
	return New(bounds, deps.UsageSource, now), nil
}

// Bounds returns the rectangle this widget occupies.
func (w *Widget) Bounds() image.Rectangle {
	return w.bounds
}

// Render draws two progress bars (5h and 7d usage windows) into frame.
func (w *Widget) Render(frame *image.Paletted) error {
	snap, err := w.source.Usage(context.Background())
	if err != nil {
		w.renderError(frame, err)
		return nil
	}

	// Fill background white.
	draw.Draw(frame, w.bounds, image.NewUniform(color.White), image.Point{}, draw.Src)

	face := basicfont.Face7x13
	bx, by := w.bounds.Min.X, w.bounds.Min.Y
	bw, bh := w.bounds.Dx(), w.bounds.Dy()

	// Split the widget vertically into two equal rows.
	rowH := bh / 2

	type row struct {
		label       string
		utilization float64
		resetsAt    time.Time
	}
	rows := []row{
		{"5h", snap.FiveHourUtilization, snap.FiveHourResetsAt},
		{"7d", snap.SevenDayUtilization, snap.SevenDayResetsAt},
	}

	for i, r := range rows {
		ry := by + i*rowH

		// Layout constants.
		labelW := font.MeasureString(face, r.label+"  ").Ceil()
		pctText := fmt.Sprintf("%d%%", int(r.utilization*100))
		pctW := font.MeasureString(face, "  "+pctText).Ceil()
		resetText := formatDuration(r.resetsAt.Sub(w.now()))

		// Progress bar dimensions.
		barX := bx + labelW
		barW := max(bw-labelW-pctW, 4)
		barY := ry + (rowH-12)/2
		barH := 12

		// Draw label.
		drawText(frame, face, bx, ry+(rowH+face.Ascent-face.Descent)/2, r.label)

		// Draw progress bar outline.
		barRect := image.Rect(barX, barY, barX+barW, barY+barH)
		drawRectOutline(frame, barRect, color.Black)

		// Draw fill.
		fillW := int(r.utilization * float64(barW-2))
		if fillW > 0 {
			fillRect := image.Rect(barX+1, barY+1, barX+1+fillW, barY+barH-1)
			draw.Draw(frame, fillRect, image.NewUniform(color.Black), image.Point{}, draw.Src)
		}

		// Draw percentage.
		drawText(frame, face, barX+barW+4, ry+(rowH+face.Ascent-face.Descent)/2, pctText)

		// Draw reset text below the bar if there's room.
		resetY := barY + barH + face.Ascent + 2
		if resetY+face.Descent <= ry+rowH {
			drawText(frame, face, barX, resetY, resetText)
		}
	}

	return nil
}

func (w *Widget) renderError(frame *image.Paletted, err error) {
	draw.Draw(frame, w.bounds, image.NewUniform(color.White), image.Point{}, draw.Src)
	face := basicfont.Face7x13
	text := "ERR: " + err.Error()
	// Truncate long error messages.
	maxChars := w.bounds.Dx() / 7
	if maxChars > 0 && len(text) > maxChars {
		text = text[:maxChars-3] + "..."
	}
	y := w.bounds.Min.Y + (w.bounds.Dy()+face.Ascent-face.Descent)/2
	drawText(frame, face, w.bounds.Min.X+2, y, text)
}

func drawText(frame *image.Paletted, face *basicfont.Face, x, y int, text string) {
	d := &font.Drawer{
		Dst:  frame,
		Src:  image.NewUniform(color.Black),
		Face: face,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(text)
}

func drawRectOutline(frame *image.Paletted, r image.Rectangle, c color.Color) {
	// Top edge.
	draw.Draw(frame, image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+1), image.NewUniform(c), image.Point{}, draw.Src)
	// Bottom edge.
	draw.Draw(frame, image.Rect(r.Min.X, r.Max.Y-1, r.Max.X, r.Max.Y), image.NewUniform(c), image.Point{}, draw.Src)
	// Left edge.
	draw.Draw(frame, image.Rect(r.Min.X, r.Min.Y, r.Min.X+1, r.Max.Y), image.NewUniform(c), image.Point{}, draw.Src)
	// Right edge.
	draw.Draw(frame, image.Rect(r.Max.X-1, r.Min.Y, r.Max.X, r.Max.Y), image.NewUniform(c), image.Point{}, draw.Src)
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		return "resetting..."
	}
	d = d.Round(time.Minute)
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60

	if hours >= 24 {
		days := hours / 24
		h := hours % 24
		if h > 0 {
			return fmt.Sprintf("resets in %dd %dh", days, h)
		}
		return fmt.Sprintf("resets in %dd", days)
	}
	if hours > 0 {
		if minutes > 0 {
			return fmt.Sprintf("resets in %dh %dm", hours, minutes)
		}
		return fmt.Sprintf("resets in %dh", hours)
	}
	return fmt.Sprintf("resets in %dm", minutes)
}
