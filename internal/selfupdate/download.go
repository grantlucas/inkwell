package selfupdate

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
)

// binaryName is the entry in each release tarball that contains the
// inkwell executable. GoReleaser sets archives.builds[].binary to
// "inkwell", so this is what every release ships.
const binaryName = "inkwell"

// Downloader fetches an asset + its checksums.txt, verifies the
// sha256, extracts the inkwell binary, and returns a temp file
// holding it. Keeping it as a struct (not a free function) so tests
// can inject an *http.Client (and a fault-injectable writeFile) via
// the constructor instead of using package globals.
type Downloader struct {
	hc        *http.Client
	writeFile func(name string, data []byte, perm os.FileMode) error
}

// NewDownloader constructs a Downloader using the given HTTP client.
// Pass http.DefaultClient (or a client with a sane timeout) in
// production; tests inject httptest.NewServer.Client().
func NewDownloader(hc *http.Client) *Downloader {
	if hc == nil {
		hc = http.DefaultClient
	}
	return &Downloader{hc: hc, writeFile: os.WriteFile}
}

// FetchVerifyExtract runs the full chain end-to-end:
//
//  1. GET checksumsURL, parse the line for assetName to recover the
//     expected sha256.
//  2. GET assetURL while streaming the body through a sha256 hasher
//     into a temp buffer.
//  3. Compare hashes — abort before touching anything if they don't
//     match.
//  4. Untar the gzipped buffer, find the "inkwell" entry, reject any
//     path-traversal entries (parent-dir refs, absolute paths).
//  5. Write the extracted bytes to a temp file with mode 0755 and
//     return its path. Caller is responsible for removing the file
//     (or os.Rename'ing it into place via the atomic-replace step).
//
// On any error the temp file (if created) is removed before
// returning, so callers don't have to clean up on the failure path.
func (d *Downloader) FetchVerifyExtract(assetURL, checksumsURL, assetName string) (string, error) {
	expectedHash, err := d.fetchExpectedHash(checksumsURL, assetName)
	if err != nil {
		return "", fmt.Errorf("fetch checksums: %w", err)
	}

	tarBytes, gotHash, err := d.fetchAndHash(assetURL)
	if err != nil {
		return "", fmt.Errorf("fetch asset: %w", err)
	}
	if gotHash != expectedHash {
		return "", fmt.Errorf("checksum mismatch for %s: expected %s, got %s",
			assetName, expectedHash, gotHash)
	}

	binBytes, err := extractInkwellBinary(tarBytes)
	if err != nil {
		return "", err
	}

	tmp, err := os.CreateTemp("", "inkwell-update-*")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	path := tmp.Name()
	_ = tmp.Close()
	// CreateTemp opens with mode 0600; os.WriteFile (the default
	// writeFile) preserves the existing mode when the file already
	// exists. Remove the placeholder so writeFile creates fresh with
	// the requested perm.
	_ = os.Remove(path)

	if err := d.writeFile(path, binBytes, 0o755); err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("write temp binary: %w", err)
	}
	return path, nil
}

// fetchExpectedHash downloads checksums.txt and returns the sha256
// recorded for assetName. Lines are "<sha>  <name>" (two spaces, per
// GoReleaser); the function is permissive — any whitespace-split
// 2-token line counts.
func (d *Downloader) fetchExpectedHash(url, assetName string) (string, error) {
	body, err := d.fetch(url)
	if err != nil {
		return "", err
	}
	scanner := bufio.NewScanner(strings.NewReader(string(body)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 {
			return "", fmt.Errorf("malformed checksums line: %q", scanner.Text())
		}
		if fields[1] == assetName {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("no checksum entry for %s", assetName)
}

// fetchAndHash downloads url into memory while computing sha256,
// returning the bytes and the hex hash.
func (d *Downloader) fetchAndHash(url string) ([]byte, string, error) {
	body, err := d.fetch(url)
	if err != nil {
		return nil, "", err
	}
	h := sha256.Sum256(body)
	return body, hex.EncodeToString(h[:]), nil
}

// fetch is a tiny http.Get wrapper that surfaces non-200 statuses as
// errors so callers don't have to repeat that check.
func (d *Downloader) fetch(url string) ([]byte, error) {
	resp, err := d.hc.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: status %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// extractInkwellBinary walks the gzipped tarball and returns the
// bytes of the entry named binaryName. Rejects any entry whose name
// resolves outside the archive root — absolute paths, "..", etc. —
// regardless of which entry it is, since the tarball itself is then
// untrustworthy.
func extractInkwellBinary(gzBytes []byte) ([]byte, error) {
	gz, err := gzip.NewReader(strings.NewReader(string(gzBytes)))
	if err != nil {
		return nil, fmt.Errorf("decompress tarball: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read tar entry: %w", err)
		}
		if !safeArchivePath(hdr.Name) {
			return nil, fmt.Errorf("rejecting tarball with unsafe path: %q", hdr.Name)
		}
		if path.Base(hdr.Name) == binaryName && hdr.Name == binaryName {
			return io.ReadAll(tr)
		}
	}
	return nil, fmt.Errorf("tarball does not contain an %q entry at its root", binaryName)
}

// safeArchivePath returns false for any tar entry that would land
// outside the archive root if naively joined: absolute paths and
// anything containing ".." as a path component.
func safeArchivePath(name string) bool {
	if strings.HasPrefix(name, "/") {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if part == ".." {
			return false
		}
	}
	return true
}
