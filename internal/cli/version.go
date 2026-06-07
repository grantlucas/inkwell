package cli

import (
	"fmt"
	"io"

	"github.com/grantlucas/inkwell/internal/buildinfo"
)

// PrintVersionShort writes the one-line `inkwell vX.Y.Z (linux/armv7)`
// summary used by `inkwell --version` and `inkwell -v`. First token
// after the program name is always the version so shell scripts can
// grep it.
func PrintVersionShort(w io.Writer) error {
	_, err := fmt.Fprintln(w, buildinfo.Get().ShortLine())
	return err
}

// PrintVersionLong writes the multi-line block used by `inkwell
// version`. Every labelled field appears so copy-pasting the output
// into a bug report fully identifies the binary.
func PrintVersionLong(w io.Writer) error {
	_, err := fmt.Fprint(w, buildinfo.Get().LongBlock())
	return err
}
