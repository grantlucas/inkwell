// Package buildinfo exposes build-time metadata stamped via
// -ldflags -X. The release pipeline (.goreleaser.yaml) supplies the
// real values; plain `go build` and `go test` get sentinels ("dev",
// "none", "unknown") so the rest of the codebase can detect an
// unstamped binary and refuse risky operations like self-update.
package buildinfo

import (
	"fmt"
	"runtime"
	"strings"
)

// These vars are overridden at link time via -X
// github.com/grantlucas/inkwell/internal/buildinfo.<Name>=<value>.
// Keep them package-level vars (not consts) — ldflags can only stamp
// strings, and only into vars.
var (
	// Version is the released semver tag, e.g. "v0.7.0". The "dev"
	// sentinel marks an unstamped build.
	Version = "dev"

	// Commit is the short git SHA the binary was built from.
	Commit = "none"

	// Date is the ISO-8601 build timestamp.
	Date = "unknown"

	// GOARM is the 32-bit arm sub-arch ("6" or "7") the binary was
	// built for. Empty on non-arm builds. runtime.GOARCH returns
	// "arm" for both armv6 and armv7, so self-update can't pick the
	// right release asset without this.
	GOARM = ""
)

// Info is a snapshot of the build metadata captured at a point in
// time. Get returns the current process's Info; tests construct
// arbitrary Infos directly to exercise formatting without touching
// the package vars.
type Info struct {
	Version string
	Commit  string
	Date    string
	GoVer   string
	GOOS    string
	GOARCH  string
	GOARM   string
}

// Get returns the build metadata for the running process.
func Get() Info {
	return Info{
		Version: Version,
		Commit:  Commit,
		Date:    Date,
		GoVer:   runtime.Version(),
		GOOS:    runtime.GOOS,
		GOARCH:  runtime.GOARCH,
		GOARM:   GOARM,
	}
}

// Platform returns "GOOS/GOARCH" with armv6/armv7 flattened so the
// string lines up 1:1 with the GoReleaser asset naming convention.
// Used by both the --version short line and the release-asset
// resolver, so they stay in lockstep.
func (i Info) Platform() string {
	arch := i.GOARCH
	if arch == "arm" && i.GOARM != "" {
		arch = "armv" + i.GOARM
	}
	return i.GOOS + "/" + arch
}

// ShortLine is the one-line summary printed by `inkwell --version`.
// First token after the program name is always the version, so shell
// scripts can grep for it.
func (i Info) ShortLine() string {
	return fmt.Sprintf("inkwell %s (%s)", i.Version, i.Platform())
}

// LongBlock is the multi-line block printed by `inkwell version`.
// Includes every field — copy-pasting this into a bug report should
// fully identify the binary.
func (i Info) LongBlock() string {
	var b strings.Builder
	fmt.Fprintf(&b, "inkwell %s\n", i.Version)
	fmt.Fprintf(&b, "  commit:   %s\n", i.Commit)
	fmt.Fprintf(&b, "  built:    %s\n", i.Date)
	fmt.Fprintf(&b, "  go:       %s\n", i.GoVer)
	fmt.Fprintf(&b, "  platform: %s\n", i.Platform())
	return b.String()
}
