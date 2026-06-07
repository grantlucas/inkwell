package selfupdate

import (
	"fmt"
	"strconv"
	"strings"
)

// devSentinel matches buildinfo.Version's default for unstamped
// builds. A dev binary self-updating is intentional: developers
// running locally still want to be able to pull the latest release
// without --force.
const devSentinel = "dev"

// compareVersions returns -1, 0, or +1 if a is older / equal /
// newer than b. Versions are "vX.Y.Z" or "X.Y.Z" — the v prefix is
// optional. The "dev" sentinel is always older than any real
// release (so the default update flow works on a dev build).
//
// Unparseable inputs produce an error rather than a fallback
// comparison so the self-update flow surfaces a clear failure
// instead of silently mis-ordering.
func compareVersions(a, b string) (int, error) {
	aDev, bDev := a == devSentinel, b == devSentinel
	switch {
	case aDev && bDev:
		return 0, nil
	case aDev:
		return -1, nil
	case bDev:
		return 1, nil
	}

	an, err := parseVersion(a)
	if err != nil {
		return 0, err
	}
	bn, err := parseVersion(b)
	if err != nil {
		return 0, err
	}
	for i := 0; i < 3; i++ {
		switch {
		case an[i] < bn[i]:
			return -1, nil
		case an[i] > bn[i]:
			return 1, nil
		}
	}
	return 0, nil
}

// parseVersion turns "vX.Y.Z" or "X.Y.Z" into a 3-int array. Any
// non-numeric component or wrong arity is an error.
//
// Pre-release and build-metadata suffixes ("-rc1", "+abc") are
// stripped before parsing — they collapse the comparison to the
// underlying X.Y.Z. That's lossy (a `v1.0.0-rc1` will compare
// equal to `v1.0.0`), but the project doesn't ship pre-releases
// today; the alternative of erroring out would break self-update
// the moment a `-rc` tag ever gets cut.
func parseVersion(s string) ([3]int, error) {
	var out [3]int
	s = strings.TrimPrefix(s, "v")
	// Drop "-rc1" / "+build.42" style suffixes.
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return out, fmt.Errorf("version %q must have 3 components", s)
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return out, fmt.Errorf("version %q component %d (%q) not numeric: %w", s, i, p, err)
		}
		out[i] = n
	}
	return out, nil
}
