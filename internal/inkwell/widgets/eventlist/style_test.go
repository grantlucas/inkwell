package eventlist_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/widgets/drawkit"
	"github.com/grantlucas/inkwell/internal/inkwell/widgets/eventlist"
)

// A style is the list shape one of the day screens draws, filled in
// with what a widget's settings choose: how many events, whether
// locations show, and the display zone. The expected styles are the
// ones bold-five, today-hero and row-agenda listed with before the
// styles existed, so a screen that switched to its style draws the
// same list.
func TestStyle_List(t *testing.T) {
	tests := []struct {
		label string
		style eventlist.Style
		want  eventlist.List
	}{
		{"stacked is bold-five's column", eventlist.StackedStyle, eventlist.List{
			MaxEvents: 4, TitleLines: 2, Gap: 8,
			Empty:        eventlist.Note{Text: "--", Centred: true},
			ShowLocation: true, Location: toronto,
		}},
		{"large is today-hero's agenda", eventlist.LargeStyle, eventlist.List{
			MaxEvents: 4, TitleLines: 2, TimeScale: 2, TitleLead: drawkit.BodyAscent(), Gap: 20, Rules: true,
			Empty:        eventlist.Note{Text: eventlist.NothingScheduled},
			ShowLocation: true, Location: toronto,
		}},
		{"inline is row-agenda's and today-hero's rows", eventlist.InlineStyle, eventlist.List{
			Layout: eventlist.Inline, MaxEvents: 4,
			Empty:        eventlist.Note{Text: eventlist.NothingScheduled},
			ShowLocation: true, Location: toronto,
		}},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			if got := tt.style.List(4, true, toronto); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Style =\n%+v\nwant\n%+v", got, tt.want)
			}
		})
	}
}

// A widget's config names its style. The names are what an operator
// types, so each one round-trips, and anything else is refused.
func TestStyleNames(t *testing.T) {
	tests := []struct {
		name string
		want eventlist.Style
		ok   bool
	}{
		{"stacked", eventlist.StackedStyle, true},
		{"large", eventlist.LargeStyle, true},
		{"inline", eventlist.InlineStyle, true},
		{"Stacked", 0, false},
		{"column", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := eventlist.StyleNames.Parse(tt.name)
			if ok != tt.ok || got != tt.want {
				t.Fatalf("Parse(%q) = %v, %v; want %v, %v", tt.name, got, ok, tt.want, tt.ok)
			}
			if ok && got.String() != tt.name {
				t.Errorf("String() = %q, want %q", got.String(), tt.name)
			}
		})
	}
	if got := eventlist.StyleNames.List(); !reflect.DeepEqual(got, []string{"stacked", "large", "inline"}) {
		t.Errorf("StyleNames = %v", got)
	}
}

// The zone is the caller's, never a default: a list written in UTC on a
// Toronto panel would put every event five hours out.
func TestStyle_ListKeepsTheZone(t *testing.T) {
	loc := time.FixedZone("X", 3600)
	if got := eventlist.InlineStyle.List(0, false, loc).Location; got != loc {
		t.Errorf("Location = %v, want %v", got, loc)
	}
}
