package daydata_test

import (
	"testing"

	"github.com/grantlucas/inkwell/internal/inkwell/widgets/daydata"
)

// shade is a widget's own enum, read from config by name.
type shade int

var shadeNames = daydata.Names[shade]{"light", "dark"}

// read holds every kind of key a widget reads for itself, with the
// defaults it starts from.
type read struct {
	Count   int
	Max     int
	On      bool
	Note    string
	NoteSet bool
	Shade   shade
}

func readKeys(raw map[string]any) (read, error) {
	r := read{Count: 2, Max: 4, Shade: 1}
	keys := daydata.ReadKeys("test-widget", raw)
	keys.Int("count", 1, 5, &r.Count)
	keys.Positive("max", &r.Max)
	keys.Bool("on", &r.On)
	r.NoteSet = keys.String("note", &r.Note)
	daydata.Choice(keys, "shade", shadeNames, &r.Shade)
	return r, keys.Err()
}

// A widget reads its own keys the same way whatever they are: an absent
// key leaves the default, a set one is checked and stored, and the first
// bad one stops the rest, named with the widget and the key.
func TestReadKeys(t *testing.T) {
	defaults := read{Count: 2, Max: 4, Shade: 1}
	tests := []struct {
		label   string
		raw     map[string]any
		want    read
		wantErr string
	}{
		{label: "nothing set keeps the defaults", raw: map[string]any{}, want: defaults},
		{
			label: "every key set",
			raw:   map[string]any{"count": 5, "max": 9, "on": true, "note": "", "shade": "light"},
			want:  read{Count: 5, Max: 9, On: true, Note: "", NoteSet: true, Shade: 0},
		},
		{label: "an int not an int", raw: map[string]any{"count": "two"}, wantErr: "test-widget: count must be an integer, got string"},
		{label: "an int below its range", raw: map[string]any{"count": 0}, wantErr: "test-widget: count must be in [1, 5], got 0"},
		{label: "an int above its range", raw: map[string]any{"count": 6}, wantErr: "test-widget: count must be in [1, 5], got 6"},
		{label: "a fraction where an int goes", raw: map[string]any{"max": 1.5}, wantErr: "test-widget: max must be a whole number, got 1.5"},
		{label: "a positive at zero", raw: map[string]any{"max": 0}, wantErr: "test-widget: max must be positive, got 0"},
		{label: "a bool not a bool", raw: map[string]any{"on": "yes"}, wantErr: "test-widget: on must be a bool, got string"},
		{label: "a string not a string", raw: map[string]any{"note": 3}, wantErr: "test-widget: note must be a string, got int"},
		{label: "a choice not a string", raw: map[string]any{"shade": 1}, wantErr: "test-widget: shade must be a string, got int"},
		{label: "a choice not one of the names", raw: map[string]any{"shade": "grey"}, wantErr: `test-widget: shade must be one of light, dark, got "grey"`},
		{
			label:   "the first bad key stops the rest",
			raw:     map[string]any{"count": 9, "on": "yes"},
			wantErr: "test-widget: count must be in [1, 5], got 9",
		},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			got, err := readKeys(tt.raw)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v", err)
			}
			if got != tt.want {
				t.Errorf("read %+v, want %+v", got, tt.want)
			}
		})
	}
}

// An enum's names are its config names in value order: a value prints
// as its name, a name parses to its value, and the list is a copy for an
// error to print.
func TestNames(t *testing.T) {
	if got := shadeNames.Name(1); got != "dark" {
		t.Errorf("Name(1) = %q, want dark", got)
	}
	if v, ok := shadeNames.Parse("light"); !ok || v != 0 {
		t.Errorf("Parse(light) = %d, %v; want 0, true", v, ok)
	}
	if _, ok := shadeNames.Parse("grey"); ok {
		t.Error("Parse(grey) named a shade")
	}
	list := shadeNames.List()
	list[0] = "changed"
	if shadeNames[0] != "light" {
		t.Error("List shares its slice with the names")
	}
}
