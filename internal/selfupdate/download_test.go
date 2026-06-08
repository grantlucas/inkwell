package selfupdate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// makeTarGz builds an in-memory .tar.gz containing files named in the
// map, with the given mode applied to each entry. Returns the gzipped
// bytes ready to serve from httptest.
func makeTarGz(t *testing.T, files map[string][]byte, mode int64) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: mode,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("WriteHeader: %v", err)
		}
		if _, err := tw.Write(content); err != nil {
			t.Fatalf("tar.Write: %v", err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar.Close: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gz.Close: %v", err)
	}
	return buf.Bytes()
}

func sha256Hex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// fixtureServer serves a tarball at /tarball and a checksums.txt at
// /checksums. The checksums file lists assetName → sha of tarBytes
// (optionally tampered to force a mismatch).
func fixtureServer(t *testing.T, assetName string, tarBytes []byte, tamper bool) (assetURL, checksumsURL string, srv *httptest.Server) {
	t.Helper()
	hash := sha256Hex(tarBytes)
	if tamper {
		hash = strings.Repeat("0", 64)
	}
	checksums := fmt.Sprintf("%s  %s\n", hash, assetName)

	mux := http.NewServeMux()
	mux.HandleFunc("/tarball", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(tarBytes)
	})
	mux.HandleFunc("/checksums", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(checksums))
	})
	srv = httptest.NewServer(mux)
	return srv.URL + "/tarball", srv.URL + "/checksums", srv
}

func TestDownloadVerifyExtract_Success(t *testing.T) {
	binary := []byte("#!/fake binary content")
	tarBytes := makeTarGz(t, map[string][]byte{"inkwell": binary, "README.md": []byte("readme")}, 0o755)
	assetURL, checksumsURL, srv := fixtureServer(t, "inkwell-linux-arm64.tar.gz", tarBytes, false)
	defer srv.Close()

	d := NewDownloader(srv.Client())
	path, err := d.FetchVerifyExtract(assetURL, checksumsURL, "inkwell-linux-arm64.tar.gz")
	if err != nil {
		t.Fatalf("FetchVerifyExtract: %v", err)
	}
	t.Cleanup(func() { os.Remove(path) })

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read extracted: %v", err)
	}
	if !bytes.Equal(got, binary) {
		t.Errorf("extracted bytes differ from fixture binary")
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if st.Mode().Perm() != 0o755 {
		t.Errorf("mode = %v, want 0o755", st.Mode().Perm())
	}
}

func TestDownloadVerifyExtract_ChecksumMismatchAborts(t *testing.T) {
	tarBytes := makeTarGz(t, map[string][]byte{"inkwell": []byte("real")}, 0o755)
	assetURL, checksumsURL, srv := fixtureServer(t, "inkwell-linux-arm64.tar.gz", tarBytes, true)
	defer srv.Close()

	d := NewDownloader(srv.Client())
	_, err := d.FetchVerifyExtract(assetURL, checksumsURL, "inkwell-linux-arm64.tar.gz")
	if err == nil {
		t.Fatal("expected checksum mismatch error")
	}
	if !strings.Contains(err.Error(), "checksum") {
		t.Errorf("error = %q, want mention of checksum", err.Error())
	}
}

func TestDownloadVerifyExtract_MissingBinary(t *testing.T) {
	// Tarball without an 'inkwell' entry.
	tarBytes := makeTarGz(t, map[string][]byte{"README.md": []byte("readme")}, 0o644)
	assetURL, checksumsURL, srv := fixtureServer(t, "inkwell-linux-arm64.tar.gz", tarBytes, false)
	defer srv.Close()

	d := NewDownloader(srv.Client())
	_, err := d.FetchVerifyExtract(assetURL, checksumsURL, "inkwell-linux-arm64.tar.gz")
	if err == nil {
		t.Fatal("expected error for missing binary")
	}
	if !strings.Contains(err.Error(), "inkwell") {
		t.Errorf("error = %q, want mention of missing binary", err.Error())
	}
}

// TestDownloadVerifyExtract_RejectsSymlinkTraversal covers the
// hdr.Linkname check: a tarball entry that is a symlink whose target
// points outside the archive root must be rejected even if the
// entry's own name is safe.
func TestDownloadVerifyExtract_RejectsSymlinkTraversal(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	// Safe-looking entry name but malicious Linkname.
	if err := tw.WriteHeader(&tar.Header{
		Name:     "inkwell",
		Linkname: "../../../../etc/passwd",
		Mode:     0o755,
		Typeflag: tar.TypeSymlink,
	}); err != nil {
		t.Fatalf("WriteHeader: %v", err)
	}
	_ = tw.Close()
	_ = gz.Close()

	assetURL, checksumsURL, srv := fixtureServer(t, "inkwell-linux-arm64.tar.gz", buf.Bytes(), false)
	defer srv.Close()

	d := NewDownloader(srv.Client())
	_, err := d.FetchVerifyExtract(assetURL, checksumsURL, "inkwell-linux-arm64.tar.gz")
	if err == nil {
		t.Fatal("expected rejection for symlink with traversing target")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Errorf("error = %q, want mention of symlink", err.Error())
	}
}

func TestDownloadVerifyExtract_RejectsPathTraversal(t *testing.T) {
	cases := []struct {
		label   string
		entries map[string][]byte
	}{
		{
			label:   "parent dir traversal",
			entries: map[string][]byte{"../inkwell": []byte("evil")},
		},
		{
			label:   "absolute path",
			entries: map[string][]byte{"/etc/passwd": []byte("evil")},
		},
		{
			label:   "nested traversal",
			entries: map[string][]byte{"foo/../../inkwell": []byte("evil")},
		},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			tarBytes := makeTarGz(t, tc.entries, 0o755)
			assetURL, checksumsURL, srv := fixtureServer(t, "inkwell-linux-arm64.tar.gz", tarBytes, false)
			defer srv.Close()

			d := NewDownloader(srv.Client())
			_, err := d.FetchVerifyExtract(assetURL, checksumsURL, "inkwell-linux-arm64.tar.gz")
			if err == nil {
				t.Fatal("expected rejection for path-traversal entry")
			}
			if !strings.Contains(err.Error(), "unsafe") && !strings.Contains(err.Error(), "path") {
				t.Errorf("error = %q, want mention of unsafe path", err.Error())
			}
		})
	}
}

func TestDownloadVerifyExtract_ChecksumsMissingAsset(t *testing.T) {
	tarBytes := makeTarGz(t, map[string][]byte{"inkwell": []byte("bin")}, 0o755)
	mux := http.NewServeMux()
	mux.HandleFunc("/tarball", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(tarBytes)
	})
	mux.HandleFunc("/checksums", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("aaaa  other-asset.tar.gz\n"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	d := NewDownloader(srv.Client())
	_, err := d.FetchVerifyExtract(srv.URL+"/tarball", srv.URL+"/checksums", "inkwell-linux-arm64.tar.gz")
	if err == nil {
		t.Fatal("expected error when asset not listed in checksums")
	}
	if !strings.Contains(err.Error(), "checksum") {
		t.Errorf("error = %q, want mention of missing checksum", err.Error())
	}
}

func TestDownloadVerifyExtract_AssetFetchFailure(t *testing.T) {
	// Checksums OK; asset URL returns 500.
	mux := http.NewServeMux()
	mux.HandleFunc("/checksums", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("aaaa  inkwell-linux-arm64.tar.gz\n"))
	})
	mux.HandleFunc("/tarball", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	d := NewDownloader(srv.Client())
	_, err := d.FetchVerifyExtract(srv.URL+"/tarball", srv.URL+"/checksums", "inkwell-linux-arm64.tar.gz")
	if err == nil {
		t.Fatal("expected error on asset 500")
	}
}

func TestDownloadVerifyExtract_ChecksumsFetchFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	d := NewDownloader(srv.Client())
	_, err := d.FetchVerifyExtract(srv.URL+"/tarball", srv.URL+"/checksums", "inkwell-linux-arm64.tar.gz")
	if err == nil {
		t.Fatal("expected error on checksums fetch failure")
	}
}

func TestDownloadVerifyExtract_MalformedTarball(t *testing.T) {
	// Bytes that aren't gzip — should fail before checksum comparison
	// matters. Tampered checksum=ZERO means we don't care that hash
	// doesn't match — we want to confirm gzip-parse fails.
	bogus := []byte("definitely not gzip")
	assetURL, checksumsURL, srv := fixtureServer(t, "inkwell-linux-arm64.tar.gz", bogus, false)
	defer srv.Close()

	d := NewDownloader(srv.Client())
	_, err := d.FetchVerifyExtract(assetURL, checksumsURL, "inkwell-linux-arm64.tar.gz")
	if err == nil {
		t.Fatal("expected gzip parse error")
	}
}

// failingReader returns an error on every Read so the scanner inside
// parseChecksumFor surfaces a scan error instead of running to EOF.
type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, fmt.Errorf("simulated read failure") }

// TestParseChecksumFor_ScannerError covers the scanner.Err() branch
// in parseChecksumFor — unreachable from production code (bytes.Reader
// never errors) but a real failure mode if a caller ever wires a
// streaming network reader through this path.
func TestParseChecksumFor_ScannerError(t *testing.T) {
	_, err := parseChecksumFor(failingReader{}, "inkwell-linux-arm64.tar.gz")
	if err == nil {
		t.Fatal("expected scan error")
	}
	if !strings.Contains(err.Error(), "read checksums") {
		t.Errorf("error = %q, want \"read checksums\"", err.Error())
	}
}

// TestNewDownloader_NilDefaults confirms the constructor tolerates a
// nil http.Client and substitutes a timeout-bounded client rather
// than http.DefaultClient — the default client has no timeout, so a
// stalled network connection would otherwise hang the updater
// indefinitely.
func TestNewDownloader_NilDefaults(t *testing.T) {
	d := NewDownloader(nil)
	if d.hc == nil {
		t.Fatal("hc must be substituted, not left nil")
	}
	if d.hc.Timeout == 0 {
		t.Errorf("nil-client fallback must set a timeout; got 0")
	}
	if d.writeFile == nil {
		t.Fatal("writeFile must have a default")
	}
}

// TestNewDownloader_ZeroTimeoutClientGetsCopy guards the
// production case where cmd/inkwell passes http.DefaultClient
// (Timeout == 0). The Downloader must enforce a timeout without
// mutating the caller's shared client — other packages that
// happen to use http.DefaultClient would otherwise inherit our
// 30s deadline.
func TestNewDownloader_ZeroTimeoutClientGetsCopy(t *testing.T) {
	shared := &http.Client{} // Timeout: 0
	d := NewDownloader(shared)

	if d.hc.Timeout == 0 {
		t.Errorf("Downloader must enforce a timeout on a zero-timeout input client")
	}
	if d.hc == shared {
		t.Errorf("Downloader must not reuse the caller's client pointer when applying a timeout")
	}
	if shared.Timeout != 0 {
		t.Errorf("caller's shared client was mutated: Timeout = %v, want 0", shared.Timeout)
	}
}

// TestNewDownloader_NonZeroTimeoutClientUsedAsIs confirms a caller
// that supplied their own positive timeout keeps that exact client
// (no copy, no override).
func TestNewDownloader_NonZeroTimeoutClientUsedAsIs(t *testing.T) {
	caller := &http.Client{Timeout: 7 * time.Second}
	d := NewDownloader(caller)

	if d.hc != caller {
		t.Errorf("non-zero-timeout client should be used as-is, got a copy")
	}
	if d.hc.Timeout != 7*time.Second {
		t.Errorf("Timeout = %v, want 7s", d.hc.Timeout)
	}
}

// TestDownloadVerifyExtract_CreateTempFails covers the create-temp
// error path by pointing TMPDIR at a nonexistent directory.
func TestDownloadVerifyExtract_CreateTempFails(t *testing.T) {
	binary := []byte("bin")
	tarBytes := makeTarGz(t, map[string][]byte{"inkwell": binary}, 0o755)
	assetURL, checksumsURL, srv := fixtureServer(t, "inkwell-linux-arm64.tar.gz", tarBytes, false)
	defer srv.Close()

	t.Setenv("TMPDIR", "/this/path/does/not/exist/anywhere")

	d := NewDownloader(srv.Client())
	_, err := d.FetchVerifyExtract(assetURL, checksumsURL, "inkwell-linux-arm64.tar.gz")
	if err == nil {
		t.Fatal("expected create-temp error")
	}
	if !strings.Contains(err.Error(), "create temp") {
		t.Errorf("error = %q, want \"create temp\"", err.Error())
	}
}

// TestDownloadVerifyExtract_WriteFileFails covers the write-file
// error path by injecting a writeFile that always errors. Also
// confirms the temp file is cleaned up on failure so /tmp doesn't
// accumulate junk.
func TestDownloadVerifyExtract_WriteFileFails(t *testing.T) {
	binary := []byte("bin")
	tarBytes := makeTarGz(t, map[string][]byte{"inkwell": binary}, 0o755)
	assetURL, checksumsURL, srv := fixtureServer(t, "inkwell-linux-arm64.tar.gz", tarBytes, false)
	defer srv.Close()

	d := NewDownloader(srv.Client())
	var triedPath string
	d.writeFile = func(name string, data []byte, perm os.FileMode) error {
		triedPath = name
		return fmt.Errorf("simulated write failure")
	}

	_, err := d.FetchVerifyExtract(assetURL, checksumsURL, "inkwell-linux-arm64.tar.gz")
	if err == nil {
		t.Fatal("expected write-file error")
	}
	if !strings.Contains(err.Error(), "write temp binary") {
		t.Errorf("error = %q, want \"write temp binary\"", err.Error())
	}
	if triedPath == "" {
		t.Fatal("writeFile stub was not called")
	}
	if _, statErr := os.Stat(triedPath); statErr == nil {
		t.Errorf("temp file %q should have been removed on failure", triedPath)
	}
}

// TestDownloadVerifyExtract_NetworkFailure covers the http.Client.Do
// error branch (different from the non-200 status branch already
// tested by ChecksumsFetchFailure / AssetFetchFailure).
func TestDownloadVerifyExtract_NetworkFailure(t *testing.T) {
	d := NewDownloader(&http.Client{})
	_, err := d.FetchVerifyExtract(
		"http://127.0.0.1:1/nope",     // closed port
		"http://127.0.0.1:1/checksums", // closed port
		"inkwell-linux-arm64.tar.gz",
	)
	if err == nil {
		t.Fatal("expected network error")
	}
}

// TestDownloadVerifyExtract_InvalidURL covers the
// http.NewRequestWithContext error branch — an unparseable URL fails
// before any network call.
func TestDownloadVerifyExtract_InvalidURL(t *testing.T) {
	d := NewDownloader(&http.Client{})
	_, err := d.FetchVerifyExtract(
		"http://invalid\x00host/asset",
		"http://invalid\x00host/checksums",
		"inkwell-linux-arm64.tar.gz",
	)
	if err == nil {
		t.Fatal("expected invalid-URL error")
	}
}

// TestDownloadVerifyExtract_CorruptTarBody covers the tar.Next
// error path: a valid gzip stream that decompresses into bytes
// that don't form a complete tar entry (truncated header).
func TestDownloadVerifyExtract_CorruptTarBody(t *testing.T) {
	// Valid gzip wrapping garbage: gzip-compress a few bytes that
	// aren't a complete tar header (tar wants 512-byte blocks).
	var gzBuf bytes.Buffer
	gz := gzip.NewWriter(&gzBuf)
	_, _ = gz.Write([]byte("not a tar header"))
	_ = gz.Close()

	assetURL, checksumsURL, srv := fixtureServer(t, "inkwell-linux-arm64.tar.gz", gzBuf.Bytes(), false)
	defer srv.Close()

	d := NewDownloader(srv.Client())
	_, err := d.FetchVerifyExtract(assetURL, checksumsURL, "inkwell-linux-arm64.tar.gz")
	if err == nil {
		t.Fatal("expected tar parse error")
	}
	if !strings.Contains(err.Error(), "tar") {
		t.Errorf("error = %q, want mention of tar", err.Error())
	}
}

func TestDownloadVerifyExtract_MalformedChecksumsLine(t *testing.T) {
	tarBytes := makeTarGz(t, map[string][]byte{"inkwell": []byte("bin")}, 0o755)
	mux := http.NewServeMux()
	mux.HandleFunc("/tarball", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(tarBytes)
	})
	mux.HandleFunc("/checksums", func(w http.ResponseWriter, _ *http.Request) {
		// Single-token line, not "<sha>  <name>".
		_, _ = w.Write([]byte("garbage-only-one-field\n"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	d := NewDownloader(srv.Client())
	_, err := d.FetchVerifyExtract(srv.URL+"/tarball", srv.URL+"/checksums", "inkwell-linux-arm64.tar.gz")
	if err == nil {
		t.Fatal("expected error for malformed checksums")
	}
}
