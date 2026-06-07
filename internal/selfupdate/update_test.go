package selfupdate

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// fakeRelease builds a Release with the standard inkwell assets.
func fakeRelease(tag string) *Release {
	return &Release{
		Tag: tag,
		assets: []asset{
			{Name: "inkwell-linux-arm64.tar.gz", URL: "https://example.invalid/arm64.tar.gz"},
			{Name: "inkwell-linux-armv6.tar.gz", URL: "https://example.invalid/armv6.tar.gz"},
			{Name: "inkwell-linux-armv7.tar.gz", URL: "https://example.invalid/armv7.tar.gz"},
			{Name: "checksums.txt", URL: "https://example.invalid/checksums.txt"},
		},
	}
}

func newFixtureUpdater() *SelfUpdater {
	return &SelfUpdater{
		CurrentVer: "v1.0.0",
		GOOS:       "linux",
		GOARCH:     "arm64",
		GOARM:      "",
		FetchLatest: func() (*Release, error) {
			return fakeRelease("v1.0.1"), nil
		},
		FetchAsset: func(assetURL, checksumsURL, name string) (string, error) {
			return "/tmp/fake-new-binary", nil
		},
		ReplaceBinary: func(srcPath string) error {
			return nil
		},
	}
}

func TestSelfUpdater_CheckOnly_PrintsAndExitsZero(t *testing.T) {
	var out bytes.Buffer
	u := newFixtureUpdater()
	if err := u.Run([]string{"--check"}, &out); err != nil {
		t.Fatalf("Run --check: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, "v1.0.0") || !strings.Contains(s, "v1.0.1") {
		t.Errorf("output should show both versions:\n%s", s)
	}
	if !strings.Contains(s, "update available") {
		t.Errorf("output should announce update availability:\n%s", s)
	}
}

func TestSelfUpdater_CheckUpToDate(t *testing.T) {
	var out bytes.Buffer
	u := newFixtureUpdater()
	u.CurrentVer = "v1.0.1" // same as latest
	if err := u.Run([]string{"--check"}, &out); err != nil {
		t.Fatalf("Run --check: %v", err)
	}
	if !strings.Contains(out.String(), "up to date") {
		t.Errorf("output should say up to date:\n%s", out.String())
	}
}

// TestSelfUpdater_DefaultSkipsWhenAtLatest covers the no-op default
// path — if current >= latest, the updater exits 0 without touching
// FetchAsset / ReplaceBinary.
func TestSelfUpdater_DefaultSkipsWhenAtLatest(t *testing.T) {
	var out bytes.Buffer
	u := newFixtureUpdater()
	u.CurrentVer = "v1.0.1"
	calledFetch, calledReplace := false, false
	u.FetchAsset = func(string, string, string) (string, error) { calledFetch = true; return "/tmp/x", nil }
	u.ReplaceBinary = func(string) error { calledReplace = true; return nil }

	if err := u.Run(nil, &out); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if calledFetch || calledReplace {
		t.Errorf("default at latest must not download or replace (fetch=%v, replace=%v)", calledFetch, calledReplace)
	}
	if !strings.Contains(out.String(), "up to date") {
		t.Errorf("output should mention up-to-date:\n%s", out.String())
	}
}

// TestSelfUpdater_DefaultUpdatesWhenBehind exercises the happy path:
// behind latest, download + replace + print systemd restart hint.
func TestSelfUpdater_DefaultUpdatesWhenBehind(t *testing.T) {
	var out bytes.Buffer
	u := newFixtureUpdater()
	var gotAsset string
	u.FetchAsset = func(_, _, name string) (string, error) {
		gotAsset = name
		return "/tmp/fake-new-binary", nil
	}
	var gotReplaceSrc string
	u.ReplaceBinary = func(src string) error {
		gotReplaceSrc = src
		return nil
	}

	if err := u.Run(nil, &out); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if gotAsset != "inkwell-linux-arm64.tar.gz" {
		t.Errorf("FetchAsset called with %q, want inkwell-linux-arm64.tar.gz", gotAsset)
	}
	if gotReplaceSrc != "/tmp/fake-new-binary" {
		t.Errorf("ReplaceBinary called with %q, want /tmp/fake-new-binary", gotReplaceSrc)
	}
	if !strings.Contains(out.String(), "systemctl restart inkwell") {
		t.Errorf("output should print restart hint:\n%s", out.String())
	}
}

func TestSelfUpdater_ForceOverridesVersionCheck(t *testing.T) {
	var out bytes.Buffer
	u := newFixtureUpdater()
	u.CurrentVer = "v1.0.1" // == latest, default would skip
	calledFetch := false
	u.FetchAsset = func(string, string, string) (string, error) { calledFetch = true; return "/tmp/x", nil }
	u.ReplaceBinary = func(string) error { return nil }

	if err := u.Run([]string{"--force"}, &out); err != nil {
		t.Fatalf("Run --force: %v", err)
	}
	if !calledFetch {
		t.Errorf("--force should cause download to run even at latest")
	}
}

func TestSelfUpdater_FetchLatestError(t *testing.T) {
	var out bytes.Buffer
	u := newFixtureUpdater()
	u.FetchLatest = func() (*Release, error) { return nil, errors.New("network down") }

	err := u.Run(nil, &out)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "network down") {
		t.Errorf("error = %v, want underlying network error", err)
	}
}

func TestSelfUpdater_UnsupportedArch(t *testing.T) {
	var out bytes.Buffer
	u := newFixtureUpdater()
	u.GOOS = "darwin"
	u.GOARCH = "arm64"

	err := u.Run(nil, &out)
	if err == nil {
		t.Fatal("expected unsupported-platform error")
	}
	if !strings.Contains(err.Error(), "darwin") {
		t.Errorf("error should mention darwin: %v", err)
	}
}

func TestSelfUpdater_AssetURLMissing(t *testing.T) {
	var out bytes.Buffer
	u := newFixtureUpdater()
	u.FetchLatest = func() (*Release, error) {
		return &Release{Tag: "v1.0.1"}, nil // no assets at all
	}

	err := u.Run(nil, &out)
	if err == nil {
		t.Fatal("expected missing-asset error")
	}
}

func TestSelfUpdater_ChecksumsURLMissing(t *testing.T) {
	var out bytes.Buffer
	u := newFixtureUpdater()
	u.FetchLatest = func() (*Release, error) {
		return &Release{
			Tag: "v1.0.1",
			assets: []asset{
				{Name: "inkwell-linux-arm64.tar.gz", URL: "https://example.invalid/x"},
				// checksums.txt missing
			},
		}, nil
	}

	err := u.Run(nil, &out)
	if err == nil {
		t.Fatal("expected missing-checksums error")
	}
	if !strings.Contains(err.Error(), "checksums") {
		t.Errorf("error should mention checksums: %v", err)
	}
}

func TestSelfUpdater_FetchAssetError(t *testing.T) {
	var out bytes.Buffer
	u := newFixtureUpdater()
	u.FetchAsset = func(string, string, string) (string, error) {
		return "", errors.New("download failed")
	}

	err := u.Run(nil, &out)
	if err == nil {
		t.Fatal("expected fetch error")
	}
}

func TestSelfUpdater_ReplaceError(t *testing.T) {
	var out bytes.Buffer
	u := newFixtureUpdater()
	u.ReplaceBinary = func(string) error { return errors.New("rename failed") }

	err := u.Run(nil, &out)
	if err == nil {
		t.Fatal("expected replace error")
	}
}

func TestSelfUpdater_UnparseableCurrentVersion(t *testing.T) {
	var out bytes.Buffer
	u := newFixtureUpdater()
	u.CurrentVer = "garbage-not-semver"
	err := u.Run(nil, &out)
	if err == nil {
		t.Fatal("expected parse error")
	}
}

// TestSelfUpdater_UnparseableLatestVersion confirms a bad release tag
// surfaces from the version-compare step, not silently downloaded.
func TestSelfUpdater_UnparseableLatestVersion(t *testing.T) {
	var out bytes.Buffer
	u := newFixtureUpdater()
	u.FetchLatest = func() (*Release, error) {
		return &Release{Tag: "not-a-version", assets: fakeRelease("v1.0.0").assets}, nil
	}
	err := u.Run(nil, &out)
	if err == nil {
		t.Fatal("expected version-parse error")
	}
}

func TestSelfUpdater_UnknownFlag(t *testing.T) {
	var out bytes.Buffer
	u := newFixtureUpdater()
	err := u.Run([]string{"--bogus"}, &out)
	if err == nil {
		t.Fatal("expected unknown-flag error")
	}
}

// TestSelfUpdater_HelpFlagEmitsUsage covers the help branch
// flag.Parse takes when --help is passed: it returns flag.ErrHelp,
// which Run propagates. Usage text is emitted via the flag package's
// default Usage handler against the writer we wired via SetOutput.
func TestSelfUpdater_HelpFlagEmitsUsage(t *testing.T) {
	var out bytes.Buffer
	u := newFixtureUpdater()
	err := u.Run([]string{"--help"}, &out)
	if err == nil {
		t.Errorf("expected flag.ErrHelp, got nil")
	}
	if !strings.Contains(out.String(), "self-update") {
		t.Errorf("usage output should mention the subcommand:\n%s", out.String())
	}
}
