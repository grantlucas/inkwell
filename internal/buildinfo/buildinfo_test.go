package buildinfo

import (
	"runtime"
	"strings"
	"testing"
)

// TestInfo_DevDefaults documents the sentinel values present in any
// build that wasn't stamped via -ldflags -X (i.e. plain `go build`,
// `go run`, `go test`). The self-update code uses Version=="dev" as
// the "refuse to update without --force" signal, so this contract has
// to be stable — never silently change the spelling.
func TestInfo_DevDefaults(t *testing.T) {
	got := Get()
	if got.Version != "dev" {
		t.Errorf("Version = %q, want \"dev\" for unstamped builds", got.Version)
	}
	if got.Commit != "none" {
		t.Errorf("Commit = %q, want \"none\"", got.Commit)
	}
	if got.Date != "unknown" {
		t.Errorf("Date = %q, want \"unknown\"", got.Date)
	}
	if got.GOOS != runtime.GOOS {
		t.Errorf("GOOS = %q, want %q", got.GOOS, runtime.GOOS)
	}
	if got.GOARCH != runtime.GOARCH {
		t.Errorf("GOARCH = %q, want %q", got.GOARCH, runtime.GOARCH)
	}
}

// TestInfo_GOARMOnlyOnArm asserts the GOARM field is only populated
// when GOARCH is "arm" — otherwise it would be a meaningless value
// in the platform line ("linux/arm64v7").
func TestInfo_GOARMOnlyOnArm(t *testing.T) {
	got := Get()
	if runtime.GOARCH == "arm" {
		// On a 32-bit arm host, GOARM has to resolve to something
		// (the build either stamped it or it falls back to "").
		// Both are acceptable here — we only assert the non-arm case.
		_ = got.GOARM
		return
	}
	if got.GOARM != "" {
		t.Errorf("GOARM = %q, want empty on non-arm host (%s)", got.GOARM, runtime.GOARCH)
	}
}

// TestInfo_Platform formats the platform suffix the self-update arch
// resolver and the --version short line both consume. armv6/armv7
// flatten into the same string GoReleaser uses for asset names, so
// downstream code never has to special-case GOARM.
func TestInfo_Platform(t *testing.T) {
	cases := []struct {
		label  string
		goos   string
		goarch string
		goarm  string
		want   string
	}{
		{label: "linux arm64", goos: "linux", goarch: "arm64", want: "linux/arm64"},
		{label: "linux armv7", goos: "linux", goarch: "arm", goarm: "7", want: "linux/armv7"},
		{label: "linux armv6", goos: "linux", goarch: "arm", goarm: "6", want: "linux/armv6"},
		{label: "linux arm no goarm", goos: "linux", goarch: "arm", want: "linux/arm"},
		{label: "darwin amd64", goos: "darwin", goarch: "amd64", want: "darwin/amd64"},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			info := Info{GOOS: tc.goos, GOARCH: tc.goarch, GOARM: tc.goarm}
			if got := info.Platform(); got != tc.want {
				t.Errorf("Platform() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestInfo_LongBlock is what `inkwell --version` prints. Every
// labelled field has to appear so a bug report copy-paste includes
// them all, and the first line must start with `inkwell vX.Y.Z` so
// scripts can grep the version.
func TestInfo_LongBlock(t *testing.T) {
	info := Info{
		Version: "v1.2.3",
		Commit:  "abc123",
		Date:    "2026-06-07",
		GOOS:    "linux",
		GOARCH:  "arm",
		GOARM:   "7",
	}
	got := info.LongBlock()
	wants := []string{
		"inkwell v1.2.3",
		"commit:",
		"abc123",
		"built:",
		"2026-06-07",
		"go:",
		"platform:",
		"linux/armv7",
	}
	for _, w := range wants {
		if !strings.Contains(got, w) {
			t.Errorf("LongBlock() missing %q\n--- got ---\n%s", w, got)
		}
	}
}
