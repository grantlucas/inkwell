package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/grantlucas/inkwell/internal/buildinfo"
)

func TestPrintVersionShort(t *testing.T) {
	var buf bytes.Buffer
	if err := PrintVersionShort(&buf); err != nil {
		t.Fatalf("PrintVersionShort: %v", err)
	}
	if !strings.HasPrefix(buf.String(), "inkwell ") {
		t.Errorf("output = %q, want prefix \"inkwell \"", buf.String())
	}
	if !strings.Contains(buf.String(), buildinfo.Get().Version) {
		t.Errorf("output should mention the build version")
	}
}

func TestPrintVersionLong(t *testing.T) {
	var buf bytes.Buffer
	if err := PrintVersionLong(&buf); err != nil {
		t.Fatalf("PrintVersionLong: %v", err)
	}
	for _, want := range []string{"inkwell ", "commit:", "built:", "go:", "platform:"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("output missing %q\n--- got ---\n%s", want, buf.String())
		}
	}
}

// errWriter forces the Fprintln/Fprint inside the version printers
// to return an error, covering the error-propagation path.
type errWriter struct{}

func (errWriter) Write([]byte) (int, error) { return 0, errors.New("simulated write failure") }

func TestPrintVersionShort_WriterError(t *testing.T) {
	if err := PrintVersionShort(errWriter{}); err == nil {
		t.Error("expected error from failing writer")
	}
}

func TestPrintVersionLong_WriterError(t *testing.T) {
	if err := PrintVersionLong(errWriter{}); err == nil {
		t.Error("expected error from failing writer")
	}
}
