package ical

import (
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
			got := unfold(strings.NewReader(tt.input))
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
