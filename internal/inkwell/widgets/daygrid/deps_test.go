package daygrid

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/weather"
	"github.com/grantlucas/inkwell/internal/inkwell/widget"
)

type offlineTransport struct{}

func (offlineTransport) Do(*http.Request) (*http.Response, error) {
	return nil, context.DeadlineExceeded
}

// A calendar widget needs both an HTTP client and the shared weather
// provider. A missing one is a wiring fault, reported by name, never
// papered over with a default.
func TestRequireDeps(t *testing.T) {
	client := offlineTransport{}
	provider := weather.NewProvider(client, time.Hour, time.Now, weather.Settings{})

	tests := []struct {
		label   string
		deps    widget.Deps
		wantErr string
	}{
		{label: "both present", deps: widget.Deps{HTTPClient: client, Weather: provider}},
		{label: "no HTTP client", deps: widget.Deps{Weather: provider}, wantErr: "bold-five: no HTTP client to fetch calendar feeds"},
		{label: "no weather provider", deps: widget.Deps{HTTPClient: client}, wantErr: "bold-five: no weather provider"},
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
