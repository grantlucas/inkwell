package daygrid

import (
	"strings"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar"
	"github.com/grantlucas/inkwell/internal/inkwell/testutil/fakehttp"
	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

// A calendar widget needs both the shared calendar module and the shared
// weather provider. A missing one is a wiring fault, reported by name,
// never papered over with a default.
func TestRequireDeps(t *testing.T) {
	tr := fakehttp.New()
	cal := calendar.NewProvider(tr, time.Now)
	provider := weather.NewProvider(tr, time.Hour, time.Now, weather.Settings{})

	tests := []struct {
		label   string
		deps    widget.Deps
		wantErr string
	}{
		{label: "both present", deps: widget.Deps{Calendar: cal, Weather: provider}},
		{label: "no calendar module", deps: widget.Deps{Weather: provider}, wantErr: "bold-five: no calendar module"},
		{label: "no weather provider", deps: widget.Deps{Calendar: cal}, wantErr: "bold-five: no weather provider"},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			err := RequireDeps("bold-five", tt.deps)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("RequireDeps: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}
