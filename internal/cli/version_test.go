package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/grantlucas/inkwell/internal/buildinfo"
)

func TestPrintVersion(t *testing.T) {
	var buf bytes.Buffer
	if err := PrintVersion(&buf); err != nil {
		t.Fatalf("PrintVersion: %v", err)
	}
	got := buf.String()
	// First line must start with `inkwell ` followed by the version
	// — that's the grep-friendly contract.
	firstLine, _, _ := strings.Cut(got, "\n")
	if !strings.HasPrefix(firstLine, "inkwell "+buildinfo.Get().Version) {
		t.Errorf("first line = %q, want prefix \"inkwell %s\"",
			firstLine, buildinfo.Get().Version)
	}
	// Bug-report fields must all be present.
	for _, want := range []string{"commit:", "built:", "go:", "platform:"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\n--- got ---\n%s", want, got)
		}
	}
}

// errWriter forces the Fprint inside PrintVersion to return an error,
// covering the error-propagation path.
type errWriter struct{}

func (errWriter) Write([]byte) (int, error) { return 0, errors.New("simulated write failure") }

func TestPrintVersion_WriterError(t *testing.T) {
	if err := PrintVersion(errWriter{}); err == nil {
		t.Error("expected error from failing writer")
	}
}
