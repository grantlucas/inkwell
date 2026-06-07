package selfupdate

import "testing"

// TestCompareVersions covers the version-ordering rule that drives
// "should we update by default?" — anything that parses as a higher
// vX.Y.Z is newer, "dev" sorts before every real release so a dev
// build sees every release as an upgrade, and an unparseable version
// surfaces as an error rather than silently sorting in some
// surprising place.
func TestCompareVersions(t *testing.T) {
	cases := []struct {
		label    string
		a, b     string
		want     int
		wantErr  bool
	}{
		{label: "equal patch", a: "v1.0.0", b: "v1.0.0", want: 0},
		{label: "patch newer", a: "v1.0.1", b: "v1.0.0", want: 1},
		{label: "patch older", a: "v1.0.0", b: "v1.0.1", want: -1},
		{label: "minor wins over patch", a: "v1.1.0", b: "v1.0.99", want: 1},
		{label: "major wins over minor", a: "v2.0.0", b: "v1.99.99", want: 1},
		{label: "numeric, not lexical: v1.0.0 > v0.10.0", a: "v1.0.0", b: "v0.10.0", want: 1},
		{label: "numeric: v0.10.0 > v0.9.99", a: "v0.10.0", b: "v0.9.99", want: 1},
		{label: "dev sorts below release", a: "dev", b: "v0.0.1", want: -1},
		{label: "release sorts above dev", a: "v0.0.1", b: "dev", want: 1},
		{label: "dev equals dev", a: "dev", b: "dev", want: 0},
		{label: "tolerates missing v prefix", a: "1.2.3", b: "v1.2.3", want: 0},
		{label: "strips pre-release suffix", a: "v1.0.0-rc1", b: "v1.0.0", want: 0},
		{label: "strips build metadata", a: "v1.2.3+build.42", b: "v1.2.3", want: 0},
		{label: "pre-release older than next patch", a: "v1.0.0-rc1", b: "v1.0.1", want: -1},
		{label: "rejects non-numeric component", a: "v1.bad.0", b: "v1.0.0", wantErr: true},
		{label: "rejects too few components", a: "v1.0", b: "v1.0.0", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			got, err := compareVersions(tc.a, tc.b)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got cmp=%d", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("compareVersions(%q, %q): %v", tc.a, tc.b, err)
			}
			if got != tc.want {
				t.Errorf("compareVersions(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
			}
		})
	}
}
