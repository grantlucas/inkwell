package selfupdate

import (
	"strings"
	"testing"
)

// TestAssetName covers the (GOOS, GOARCH, GOARM) → release-asset
// mapping. The expected strings must match GoReleaser's
// name_template in .goreleaser.yaml exactly — drift between the two
// means self-update would 404 on every Pi until the next release.
func TestAssetName(t *testing.T) {
	cases := []struct {
		label  string
		goos   string
		goarch string
		goarm  string
		want   string
		errSub string
	}{
		{label: "linux arm64", goos: "linux", goarch: "arm64", want: "inkwell-linux-arm64.tar.gz"},
		{label: "linux armv6", goos: "linux", goarch: "arm", goarm: "6", want: "inkwell-linux-armv6.tar.gz"},
		{label: "linux armv7", goos: "linux", goarch: "arm", goarm: "7", want: "inkwell-linux-armv7.tar.gz"},
		{label: "rejects darwin", goos: "darwin", goarch: "arm64", errSub: "darwin/arm64"},
		{label: "rejects linux amd64", goos: "linux", goarch: "amd64", errSub: "linux/amd64"},
		{label: "rejects linux arm goarm 5", goos: "linux", goarch: "arm", goarm: "5", errSub: "armv5"},
		{label: "rejects linux arm no goarm", goos: "linux", goarch: "arm", errSub: "GOARM"},
		{label: "rejects windows", goos: "windows", goarch: "amd64", errSub: "windows/amd64"},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			got, err := AssetName(tc.goos, tc.goarch, tc.goarm)
			if tc.errSub != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got name %q", tc.errSub, got)
				}
				if !strings.Contains(err.Error(), tc.errSub) {
					t.Errorf("error = %q, want substring %q", err.Error(), tc.errSub)
				}
				return
			}
			if err != nil {
				t.Fatalf("AssetName: %v", err)
			}
			if got != tc.want {
				t.Errorf("AssetName = %q, want %q", got, tc.want)
			}
		})
	}
}
