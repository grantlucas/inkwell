package eventlist_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daygrid"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/eventlist"
)

// A preset is the list shape one of the day screens draws, filled in
// with what a widget's settings choose: how many events, whether
// locations show, and the display zone. The expected styles are the
// ones bold-five, today-hero and row-agenda listed with before the
// presets existed, so a screen that switched to its preset draws the
// same list.
func TestPreset_Style(t *testing.T) {
	tests := []struct {
		label  string
		preset eventlist.Preset
		want   eventlist.Style
	}{
		{"stacked is bold-five's column", eventlist.PresetStacked, eventlist.Style{
			MaxEvents: 4, TitleLines: 2, Gap: 8,
			Empty:        eventlist.Note{Text: "--", Centred: true},
			ShowLocation: true, Location: toronto,
		}},
		{"large is today-hero's agenda", eventlist.PresetLarge, eventlist.Style{
			MaxEvents: 4, TitleLines: 2, TimeScale: 2, TitleLead: daygrid.BodyAscent(), Gap: 20, Rules: true,
			Empty:        eventlist.Note{Text: eventlist.NothingScheduled},
			ShowLocation: true, Location: toronto,
		}},
		{"inline is row-agenda's and today-hero's rows", eventlist.PresetInline, eventlist.Style{
			Layout: eventlist.Inline, MaxEvents: 4,
			Empty:        eventlist.Note{Text: eventlist.NothingScheduled},
			ShowLocation: true, Location: toronto,
		}},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			if got := tt.preset.Style(4, true, toronto); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Style =\n%+v\nwant\n%+v", got, tt.want)
			}
		})
	}
}

// A widget's config names its preset. The names are what an operator
// types, so each one round-trips, and anything else is refused.
func TestParsePreset(t *testing.T) {
	tests := []struct {
		name string
		want eventlist.Preset
		ok   bool
	}{
		{"stacked", eventlist.PresetStacked, true},
		{"large", eventlist.PresetLarge, true},
		{"inline", eventlist.PresetInline, true},
		{"Stacked", 0, false},
		{"column", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := eventlist.ParsePreset(tt.name)
			if ok != tt.ok || got != tt.want {
				t.Fatalf("ParsePreset(%q) = %v, %v; want %v, %v", tt.name, got, ok, tt.want, tt.ok)
			}
			if ok && got.String() != tt.name {
				t.Errorf("String() = %q, want %q", got.String(), tt.name)
			}
		})
	}
	if got := eventlist.PresetNames(); !reflect.DeepEqual(got, []string{"stacked", "large", "inline"}) {
		t.Errorf("PresetNames = %v", got)
	}
}

// The zone is the caller's, never a default: a list written in UTC on a
// Toronto panel would put every event five hours out.
func TestPreset_StyleKeepsTheZone(t *testing.T) {
	loc := time.FixedZone("X", 3600)
	if got := eventlist.PresetInline.Style(0, false, loc).Location; got != loc {
		t.Errorf("Location = %v, want %v", got, loc)
	}
}
