package daydata_test

import (
	"bytes"
	"image"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daydata"
)

// A widget given less room than it needs draws nothing, and says so in
// the log: its widget, the bounds it has, the size it needs and, when the
// size depends on its settings, what for.
func TestFits(t *testing.T) {
	need := image.Pt(200, 100)
	tests := []struct {
		label   string
		bounds  image.Rectangle
		forWhat string
		want    bool
		wantLog string
	}{
		{label: "exactly the size", bounds: image.Rect(10, 10, 210, 110), want: true},
		{label: "larger", bounds: image.Rect(0, 0, 800, 480), want: true},
		{
			label: "too narrow", bounds: image.Rect(0, 0, 199, 100),
			wantLog: "test-widget: bounds are 199x100, need at least 200x100 — drawing nothing",
		},
		{
			label: "too short, for a setting", bounds: image.Rect(0, 0, 200, 99), forWhat: "4 days",
			wantLog: "test-widget: bounds are 200x99, need at least 200x100 for 4 days — drawing nothing",
		},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			var buf bytes.Buffer
			log.SetOutput(&buf)
			flags := log.Flags()
			log.SetFlags(0)
			t.Cleanup(func() { log.SetOutput(os.Stderr); log.SetFlags(flags) })

			if got := daydata.Fits("test-widget", tt.bounds, need, tt.forWhat); got != tt.want {
				t.Errorf("Fits = %v, want %v", got, tt.want)
			}
			if got := strings.TrimSpace(buf.String()); got != tt.wantLog {
				t.Errorf("logged %q, want %q", got, tt.wantLog)
			}
		})
	}
}
