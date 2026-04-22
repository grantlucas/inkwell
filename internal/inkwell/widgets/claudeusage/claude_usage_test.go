package claudeusage

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/testutil"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

type stubUsageSource struct {
	snapshot widget.UsageSnapshot
	err      error
}

func (s *stubUsageSource) Usage(_ context.Context) (widget.UsageSnapshot, error) {
	return s.snapshot, s.err
}

var (
	testBounds = image.Rect(0, 0, 400, 80)
	testNow    = time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC)
	palette    = color.Palette{color.White, color.Black}
)

func fixedClock() func() time.Time {
	return func() time.Time { return testNow }
}

func testSnapshot() widget.UsageSnapshot {
	return widget.UsageSnapshot{
		FiveHourUtilization: 0.42,
		FiveHourResetsAt:    testNow.Add(2*time.Hour + 15*time.Minute),
		SevenDayUtilization: 0.15,
		SevenDayResetsAt:    testNow.Add(6*24*time.Hour + 12*time.Hour),
	}
}

func TestFactory_NilUsageSourceErrors(t *testing.T) {
	_, err := Factory(testBounds, nil, widget.Deps{})
	if err == nil {
		t.Fatal("expected error for nil UsageSource")
	}
}

func TestFactory_Success(t *testing.T) {
	deps := widget.Deps{
		UsageSource: &stubUsageSource{snapshot: testSnapshot()},
		Now:         fixedClock(),
	}
	w, err := Factory(testBounds, nil, deps)
	if err != nil {
		t.Fatalf("Factory: %v", err)
	}
	if w == nil {
		t.Fatal("expected non-nil widget")
	}
}

func TestFactory_NilNowUsesDefault(t *testing.T) {
	deps := widget.Deps{
		UsageSource: &stubUsageSource{snapshot: testSnapshot()},
	}
	w, err := Factory(testBounds, nil, deps)
	if err != nil {
		t.Fatalf("Factory: %v", err)
	}
	// Should not panic on render.
	frame := image.NewPaletted(testBounds, palette)
	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}
}

func TestWidget_Bounds(t *testing.T) {
	w := New(testBounds, &stubUsageSource{}, fixedClock())
	if got := w.Bounds(); got != testBounds {
		t.Errorf("Bounds() = %v, want %v", got, testBounds)
	}
}

func TestWidget_RenderProducesNonBlankOutput(t *testing.T) {
	w := New(testBounds, &stubUsageSource{snapshot: testSnapshot()}, fixedClock())
	frame := image.NewPaletted(testBounds, palette)

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
		t.Error("Render produced a blank frame")
	}
}

func TestWidget_DifferentValuesProduceDifferentOutput(t *testing.T) {
	snap1 := testSnapshot()
	snap2 := widget.UsageSnapshot{
		FiveHourUtilization: 0.95,
		FiveHourResetsAt:    testNow.Add(30 * time.Minute),
		SevenDayUtilization: 0.80,
		SevenDayResetsAt:    testNow.Add(24 * time.Hour),
	}

	render := func(snap widget.UsageSnapshot) []byte {
		w := New(testBounds, &stubUsageSource{snapshot: snap}, fixedClock())
		frame := image.NewPaletted(testBounds, palette)
		if err := w.Render(frame); err != nil {
			t.Fatalf("Render: %v", err)
		}
		out := make([]byte, len(frame.Pix))
		copy(out, frame.Pix)
		return out
	}

	pix1 := render(snap1)
	pix2 := render(snap2)
	if bytes.Equal(pix1, pix2) {
		t.Error("different snapshots produced identical frames")
	}
}

func TestWidget_RenderSourceError(t *testing.T) {
	w := New(testBounds, &stubUsageSource{err: errors.New("connection refused")}, fixedClock())
	frame := image.NewPaletted(testBounds, palette)

	// Should not return an error — renders error text instead.
	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}

	// Should have some non-white pixels (error text).
	hasBlack := false
	for _, px := range frame.Pix {
		if px != 0 {
			hasBlack = true
			break
		}
	}
	if !hasBlack {
		t.Error("error render produced a blank frame")
	}
}

func TestWidget_RenderGolden(t *testing.T) {
	w := New(testBounds, &stubUsageSource{snapshot: testSnapshot()}, fixedClock())
	frame := image.NewPaletted(testBounds, palette)

	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}

	testutil.AssertGoldenPNG(t, frame)
}

func TestWidget_RenderErrorGolden(t *testing.T) {
	w := New(testBounds, &stubUsageSource{err: errors.New("timeout")}, fixedClock())
	frame := image.NewPaletted(testBounds, palette)

	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}

	testutil.AssertGoldenPNG(t, frame)
}

func TestWidget_ZeroUtilization(t *testing.T) {
	snap := widget.UsageSnapshot{
		FiveHourUtilization: 0,
		FiveHourResetsAt:    testNow.Add(5 * time.Hour),
		SevenDayUtilization: 0,
		SevenDayResetsAt:    testNow.Add(7 * 24 * time.Hour),
	}
	w := New(testBounds, &stubUsageSource{snapshot: snap}, fixedClock())
	frame := image.NewPaletted(testBounds, palette)

	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}

	// Should still render labels and outlines.
	hasBlack := false
	for _, px := range frame.Pix {
		if px != 0 {
			hasBlack = true
			break
		}
	}
	if !hasBlack {
		t.Error("zero utilization render produced a blank frame")
	}
}

func TestWidget_NarrowBounds(t *testing.T) {
	// Very narrow bounds to exercise barW < 4 branch.
	narrow := image.Rect(0, 0, 30, 40)
	snap := testSnapshot()
	w := New(narrow, &stubUsageSource{snapshot: snap}, fixedClock())
	frame := image.NewPaletted(narrow, palette)

	// Should not panic.
	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}
}

func TestWidget_TallBoundsShowsResetText(t *testing.T) {
	// Tall enough that reset text fits below the progress bar.
	tall := image.Rect(0, 0, 400, 120)
	w := New(tall, &stubUsageSource{snapshot: testSnapshot()}, fixedClock())
	frame := image.NewPaletted(tall, palette)

	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{-time.Minute, "resetting..."},
		{0, "resets in 0m"},
		{30 * time.Minute, "resets in 30m"},
		{2*time.Hour + 15*time.Minute, "resets in 2h 15m"},
		{5 * time.Hour, "resets in 5h"},
		{24 * time.Hour, "resets in 1d"},
		{6*24*time.Hour + 12*time.Hour, "resets in 6d 12h"},
		{48 * time.Hour, "resets in 2d"},
	}
	for _, tt := range tests {
		got := formatDuration(tt.d)
		if got != tt.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestWidget_LongErrorTruncation(t *testing.T) {
	longErr := errors.New("this is a very long error message that should get truncated to fit within the widget bounds area")
	w := New(testBounds, &stubUsageSource{err: longErr}, fixedClock())
	frame := image.NewPaletted(testBounds, palette)

	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}
}

func TestWidget_TinyBoundsErrorTruncation(t *testing.T) {
	// Bounds so small that maxChars <= 3, which would previously panic
	// with a negative slice index.
	tiny := image.Rect(0, 0, 14, 20)
	w := New(tiny, &stubUsageSource{err: errors.New("fail")}, fixedClock())
	frame := image.NewPaletted(tiny, palette)

	// Should not panic.
	if err := w.Render(frame); err != nil {
		t.Fatalf("Render: %v", err)
	}
}
