// Package selfupdate implements the in-place binary upgrade flow:
// resolving the right release asset for the current platform,
// fetching it from GitHub Releases, verifying its sha256 against
// checksums.txt, and atomically swapping the running binary.
package selfupdate

import "fmt"

// AssetName maps a (GOOS, GOARCH, GOARM) triple to the tarball
// name produced by the project's GoReleaser config. Stays in
// lockstep with .goreleaser.yaml's archives.name_template — if
// either drifts the updater 404s on every install until the next
// release fixes the name.
//
// Only linux is supported (the panel only runs on a Pi); armv6 and
// armv7 are split by GOARM because runtime.GOARCH returns "arm"
// for both. Any other combination is a clear error rather than a
// silent fallback.
func AssetName(goos, goarch, goarm string) (string, error) {
	if goos != "linux" {
		return "", fmt.Errorf("unsupported platform %s/%s — self-update only supports linux", goos, goarch)
	}
	switch goarch {
	case "arm64":
		return "inkwell-linux-arm64.tar.gz", nil
	case "arm":
		switch goarm {
		case "":
			return "", fmt.Errorf("linux/arm requires GOARM to pick the right asset (got empty); rebuild with -X buildinfo.GOARM=6 or =7")
		case "6", "7":
			return fmt.Sprintf("inkwell-linux-armv%s.tar.gz", goarm), nil
		default:
			return "", fmt.Errorf("unsupported arm sub-arch: armv%s (only armv6 and armv7 are released)", goarm)
		}
	default:
		return "", fmt.Errorf("unsupported platform %s/%s — self-update only supports linux/arm64, linux/armv6, linux/armv7", goos, goarch)
	}
}
