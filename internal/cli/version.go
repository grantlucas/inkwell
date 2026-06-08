package cli

import (
	"fmt"
	"io"

	"github.com/grantlucas/inkwell/internal/buildinfo"
)

// PrintVersion writes the build metadata block used by
// `inkwell --version` / `-v`. First line is always
// `inkwell vX.Y.Z` so shell scripts can grep the version; the
// remaining lines (commit, build date, Go runtime, platform) are
// copy-paste fodder for bug reports.
func PrintVersion(w io.Writer) error {
	_, err := fmt.Fprint(w, buildinfo.Get().LongBlock())
	return err
}
