package selfupdate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// e2eFixture spins up an httptest server that mimics the GitHub
// releases/latest endpoint plus the asset and checksums file URLs
// referenced from that payload. The fixture's tarball contains a
// fake "inkwell" binary; success means after Run, the on-disk
// target binary has been replaced byte-for-byte with that fixture.
type e2eFixture struct {
	srv         *httptest.Server
	repo        string
	target      string
	fixtureBin  []byte
	tampered    bool
	missingArch string // if set, omit this asset name from the release JSON
}

func newE2EFixture(t *testing.T) *e2eFixture {
	t.Helper()
	dir := t.TempDir()
	target := filepath.Join(dir, "inkwell")
	if err := os.WriteFile(target, []byte("OLD-BINARY"), 0o755); err != nil {
		t.Fatalf("seed target: %v", err)
	}
	return &e2eFixture{
		repo:       "owner/repo",
		target:     target,
		fixtureBin: []byte("NEW-BINARY-FROM-RELEASE\n"),
	}
}

func (f *e2eFixture) start(t *testing.T) {
	t.Helper()
	mux := http.NewServeMux()

	// The release JSON references absolute URLs back at this server.
	srv := httptest.NewServer(mux)
	f.srv = srv

	tarBytes := makeTarGz(t, map[string][]byte{"inkwell": f.fixtureBin}, 0o755)

	// Build the checksums.txt content (real sha or tampered).
	hash := sha256Hex(tarBytes)
	if f.tampered {
		hash = strings.Repeat("0", 64)
	}
	checksums := fmt.Sprintf("%s  inkwell-linux-arm64.tar.gz\n", hash)

	mux.HandleFunc("/repos/"+f.repo+"/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		assetsList := []map[string]string{
			{"name": "checksums.txt", "browser_download_url": srv.URL + "/checksums.txt"},
		}
		// Include arm64 asset unless the test wants it missing.
		if f.missingArch != "inkwell-linux-arm64.tar.gz" {
			assetsList = append(assetsList, map[string]string{
				"name":                 "inkwell-linux-arm64.tar.gz",
				"browser_download_url": srv.URL + "/asset.tar.gz",
			})
		}
		payload := map[string]any{
			"tag_name": "v9.9.9",
			"assets":   assetsList,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	})
	mux.HandleFunc("/asset.tar.gz", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(tarBytes)
	})
	mux.HandleFunc("/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(checksums))
	})
}

func (f *e2eFixture) updater(t *testing.T) *SelfUpdater {
	t.Helper()
	gh := NewGitHubClient(f.repo,
		WithBaseURL(f.srv.URL),
		WithHTTPClient(f.srv.Client()),
	)
	dl := NewDownloader(f.srv.Client())
	rp := NewReplacer()

	return &SelfUpdater{
		CurrentVer:    "v0.0.1",
		GOOS:          "linux",
		GOARCH:        "arm64",
		FetchLatest:   gh.LatestRelease,
		FetchAsset:    dl.FetchVerifyExtract,
		ReplaceBinary: func(srcPath string) error { return rp.ReplaceAt(f.target, srcPath) },
	}
}

// TestSelfUpdater_E2E_Success drives the full pipeline: fetch
// release JSON, download + verify the tarball, atomically rename
// over the fake "installed" binary. Final assertion is the disk
// content matching the fixture byte-for-byte — the strongest
// guarantee that no step silently corrupted the bytes.
func TestSelfUpdater_E2E_Success(t *testing.T) {
	f := newE2EFixture(t)
	f.start(t)
	defer f.srv.Close()

	var out bytes.Buffer
	if err := f.updater(t).Run(nil, &out); err != nil {
		t.Fatalf("Run: %v", err)
	}
	got, err := os.ReadFile(f.target)
	if err != nil {
		t.Fatalf("read updated target: %v", err)
	}
	if !bytes.Equal(got, f.fixtureBin) {
		t.Errorf("target bytes (%q) != fixture (%q)", got, f.fixtureBin)
	}
	if !strings.Contains(out.String(), "systemctl restart") {
		t.Errorf("expected restart hint in output:\n%s", out.String())
	}
}

// TestSelfUpdater_E2E_TamperedChecksumAborts confirms the verify
// step rejects a corrupted artifact before any disk write touches
// the install path. After the failure the original target must be
// byte-identical to what we seeded.
func TestSelfUpdater_E2E_TamperedChecksumAborts(t *testing.T) {
	f := newE2EFixture(t)
	f.tampered = true
	f.start(t)
	defer f.srv.Close()

	originalBytes, _ := os.ReadFile(f.target)

	var out bytes.Buffer
	err := f.updater(t).Run(nil, &out)
	if err == nil {
		t.Fatal("expected checksum mismatch error")
	}
	if !strings.Contains(err.Error(), "checksum") {
		t.Errorf("error = %q, want mention of checksum", err.Error())
	}

	got, _ := os.ReadFile(f.target)
	if !bytes.Equal(got, originalBytes) {
		t.Errorf("target was modified despite checksum mismatch: %q", got)
	}
}

// TestSelfUpdater_E2E_WrongArchError covers the resolver/missing-
// asset path: a release without the expected asset surfaces a
// clear error rather than a silent skip.
func TestSelfUpdater_E2E_WrongArchError(t *testing.T) {
	f := newE2EFixture(t)
	f.missingArch = "inkwell-linux-arm64.tar.gz"
	f.start(t)
	defer f.srv.Close()

	var out bytes.Buffer
	err := f.updater(t).Run(nil, &out)
	if err == nil {
		t.Fatal("expected missing-asset error")
	}
	if !strings.Contains(err.Error(), "inkwell-linux-arm64.tar.gz") {
		t.Errorf("error = %q, want mention of missing asset name", err.Error())
	}
}
