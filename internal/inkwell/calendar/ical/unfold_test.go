package ical

import (
	"errors"
	"strings"
	"testing"
)

func TestUnfold(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "no folding",
			input: "SUMMARY:Hello\r\nDTSTART:20260101\r\n",
			want:  []string{"SUMMARY:Hello", "DTSTART:20260101"},
		},
		{
			name:  "space continuation",
			input: "SUMMARY:Hello\r\n  World\r\n",
			want:  []string{"SUMMARY:Hello World"}, // leading space stripped, second space is content
		},
		{
			name:  "tab continuation",
			input: "SUMMARY:Hello\r\n\tWorld\r\n",
			want:  []string{"SUMMARY:HelloWorld"},
		},
		{
			name:  "multiple continuations",
			input: "SUMMARY:One\r\n Two\r\n Three\r\n",
			want:  []string{"SUMMARY:OneTwoThree"},
		},
		{
			name:  "bare LF line endings",
			input: "SUMMARY:Hello\n World\n",
			want:  []string{"SUMMARY:HelloWorld"},
		},
		{
			name:  "empty input",
			input: "",
			want:  nil,
		},
		{
			name:  "blank lines ignored",
			input: "SUMMARY:Hello\r\n\r\nDTSTART:20260101\r\n",
			want:  []string{"SUMMARY:Hello", "DTSTART:20260101"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := unfold(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %d lines, want %d\ngot:  %q\nwant: %q", len(got), len(tt.want), got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("line %d = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// errReader returns an error mid-stream so we can verify unfold surfaces
// scanner failures instead of silently truncating the result.
type errReader struct {
	data []byte
	pos  int
}

func (e *errReader) Read(p []byte) (int, error) {
	if e.pos >= len(e.data) {
		return 0, errors.New("simulated read failure")
	}
	n := copy(p, e.data[e.pos:])
	e.pos += n
	return n, nil
}

func TestUnfold_ScannerError(t *testing.T) {
	r := &errReader{data: []byte("SUMMARY:Hello\n")}
	_, err := unfold(r)
	if err == nil {
		t.Fatal("expected error from failing reader, got nil")
	}
	if !strings.Contains(err.Error(), "simulated read failure") {
		t.Errorf("error = %q, want it to wrap simulated read failure", err.Error())
	}
}

// Ensure unfold returns nil on a no-op reader without spurious error.
func TestUnfold_EOFOnly(t *testing.T) {
	got, err := unfold(strings.NewReader(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("got %q, want nil", got)
	}
}
