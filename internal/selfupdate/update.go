package selfupdate

import (
	"flag"
	"fmt"
	"io"
)

// SelfUpdater is the orchestrator that wires the GitHub releases
// client, the download/verify step, and the atomic-replace step
// behind the `inkwell self-update` subcommand. All dependencies are
// held as function fields rather than concrete struct values so the
// CLI handler in cmd/inkwell can install the real implementations
// while tests inject stubs without spinning up an httptest server
// for orchestrator-level checks (the end-to-end test in
// internal/selfupdate covers that integration separately).
type SelfUpdater struct {
	// CurrentVer is the running binary's version, typically
	// buildinfo.Version. "dev" means an unstamped build and is
	// treated as older than every real release.
	CurrentVer string

	// GOOS / GOARCH / GOARM describe the running binary. GOARM is
	// load-bearing for arm builds (it picks between armv6 and
	// armv7 release assets).
	GOOS, GOARCH, GOARM string

	// FetchLatest returns the latest release. In production, wrap
	// (*GitHubClient).LatestRelease.
	FetchLatest func() (*Release, error)

	// FetchAsset downloads, sha256-verifies, and extracts the
	// inkwell binary from the release tarball, returning a temp
	// file path. In production, wrap (*Downloader).FetchVerifyExtract.
	FetchAsset func(assetURL, checksumsURL, assetName string) (string, error)

	// ReplaceBinary atomically replaces the running binary with
	// the bytes at srcPath. In production, wrap (*Replacer).Replace.
	ReplaceBinary func(srcPath string) error
}

// Run parses the self-update flags and drives the upgrade flow.
// Returns nil on success — the caller should print no further
// output and exit 0. Returns an error if any step failed, with a
// message suitable for stderr.
func (s *SelfUpdater) Run(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("self-update", flag.ContinueOnError)
	fs.SetOutput(stdout)
	check := fs.Bool("check", false, "print current vs latest and exit 0 without writing anything")
	force := fs.Bool("force", false, "apply update even when current >= latest")
	if err := fs.Parse(args); err != nil {
		return err
	}

	rel, err := s.FetchLatest()
	if err != nil {
		return fmt.Errorf("fetch latest release: %w", err)
	}

	cmp, err := compareVersions(s.CurrentVer, rel.Tag)
	if err != nil {
		return fmt.Errorf("compare versions current=%q latest=%q: %w", s.CurrentVer, rel.Tag, err)
	}

	if *check {
		if cmp < 0 {
			fmt.Fprintf(stdout, "current=%s latest=%s — update available\n", s.CurrentVer, rel.Tag)
		} else {
			fmt.Fprintf(stdout, "current=%s latest=%s — up to date\n", s.CurrentVer, rel.Tag)
		}
		return nil
	}

	if cmp >= 0 && !*force {
		fmt.Fprintf(stdout, "current=%s — up to date\n", s.CurrentVer)
		return nil
	}

	assetName, err := AssetName(s.GOOS, s.GOARCH, s.GOARM)
	if err != nil {
		return err
	}
	assetURL, err := rel.AssetURL(assetName)
	if err != nil {
		return fmt.Errorf("locate %s in release: %w", assetName, err)
	}
	checksumsURL, err := rel.ChecksumsURL()
	if err != nil {
		return fmt.Errorf("locate checksums.txt in release: %w", err)
	}

	srcPath, err := s.FetchAsset(assetURL, checksumsURL, assetName)
	if err != nil {
		return fmt.Errorf("download release: %w", err)
	}

	if err := s.ReplaceBinary(srcPath); err != nil {
		return fmt.Errorf("replace binary: %w", err)
	}

	fmt.Fprintf(stdout, "updated to %s — run `sudo systemctl restart inkwell.service` to pick up the new binary\n", rel.Tag)
	return nil
}
