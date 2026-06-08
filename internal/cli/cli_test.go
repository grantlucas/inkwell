package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// captureRun returns an Options whose handlers record which one was
// invoked and with what args, so tests can assert dispatch behavior
// without involving the real handlers.
type capture struct {
	calledApp        bool
	appConfigPath    string
	calledSelfUpdate bool
	selfUpdateArgs   []string
	calledVersion    bool
}

func captureOpts(c *capture, stdout, stderr *bytes.Buffer) Options {
	return Options{
		Stdout: stdout,
		Stderr: stderr,
		RunApp: func(configPath string) error {
			c.calledApp = true
			c.appConfigPath = configPath
			return nil
		},
		SelfUpdate: func(args []string) error {
			c.calledSelfUpdate = true
			c.selfUpdateArgs = args
			return nil
		},
		Version: func() error {
			c.calledVersion = true
			return nil
		},
	}
}

func TestRun_Dispatch(t *testing.T) {
	cases := []struct {
		label          string
		args           []string
		wantApp        bool
		wantConfig     string
		wantSelfUpdate bool
		wantSelfArgs   []string
		wantVersion    bool
		wantExit       int
		wantStderrSub  string
	}{
		{
			label:      "no args runs app with default config",
			args:       nil,
			wantApp:    true,
			wantConfig: "inkwell.yaml",
		},
		{
			label:      "yaml path runs app with that path",
			args:       []string{"custom.yaml"},
			wantApp:    true,
			wantConfig: "custom.yaml",
		},
		{
			label:        "self-update subcommand dispatches without inner args",
			args:         []string{"self-update"},
			wantSelfUpdate: true,
			wantSelfArgs: []string{},
		},
		{
			label:          "self-update passes through inner args",
			args:           []string{"self-update", "--check"},
			wantSelfUpdate: true,
			wantSelfArgs:   []string{"--check"},
		},
		{
			label:         "unknown subcommand exits non-zero with usage",
			args:          []string{"nonsense"},
			wantExit:      2,
			wantStderrSub: "usage:",
		},
		{
			label:         "unknown flag exits non-zero with usage",
			args:          []string{"--bogus"},
			wantExit:      2,
			wantStderrSub: "usage:",
		},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			var c capture
			var stdout, stderr bytes.Buffer
			got := Run(tc.args, captureOpts(&c, &stdout, &stderr))

			if got != tc.wantExit {
				t.Errorf("exit = %d, want %d (stderr=%q)", got, tc.wantExit, stderr.String())
			}
			if c.calledApp != tc.wantApp {
				t.Errorf("calledApp = %v, want %v", c.calledApp, tc.wantApp)
			}
			if tc.wantApp && c.appConfigPath != tc.wantConfig {
				t.Errorf("appConfigPath = %q, want %q", c.appConfigPath, tc.wantConfig)
			}
			if c.calledSelfUpdate != tc.wantSelfUpdate {
				t.Errorf("calledSelfUpdate = %v, want %v", c.calledSelfUpdate, tc.wantSelfUpdate)
			}
			if tc.wantSelfUpdate && !stringSliceEq(c.selfUpdateArgs, tc.wantSelfArgs) {
				t.Errorf("selfUpdateArgs = %v, want %v", c.selfUpdateArgs, tc.wantSelfArgs)
			}
			if c.calledVersion != tc.wantVersion {
				t.Errorf("called Version = %v, want %v", c.calledVersion, tc.wantVersion)
			}
			if tc.wantStderrSub != "" && !strings.Contains(stderr.String(), tc.wantStderrSub) {
				t.Errorf("stderr = %q, want substring %q", stderr.String(), tc.wantStderrSub)
			}
		})
	}
}

// TestRun_HandlerErrorBecomesExitOne confirms a handler error surfaces
// as exit 1 with the error message on stderr. The router doesn't know
// about handler-specific failure modes — it just propagates them.
func TestRun_HandlerErrorBecomesExitOne(t *testing.T) {
	var c capture
	var stdout, stderr bytes.Buffer
	opts := captureOpts(&c, &stdout, &stderr)
	opts.RunApp = func(string) error { return errors.New("boom") }

	got := Run(nil, opts)
	if got != 1 {
		t.Errorf("exit = %d, want 1", got)
	}
	if !strings.Contains(stderr.String(), "boom") {
		t.Errorf("stderr = %q, want \"boom\"", stderr.String())
	}
}

// TestRun_VersionFlagShortCircuits proves --version / -v are handled
// at the router level — they win regardless of what other args
// follow them, so e.g. `inkwell --version some.yaml` prints version
// rather than trying to start the dashboard.
func TestRun_VersionFlagShortCircuits(t *testing.T) {
	cases := [][]string{
		{"--version"},
		{"-v"},
		{"--version", "ignored.yaml"},
	}
	for _, args := range cases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var c capture
			var stdout, stderr bytes.Buffer
			got := Run(args, captureOpts(&c, &stdout, &stderr))
			if got != 0 {
				t.Errorf("exit = %d, want 0", got)
			}
			if !c.calledVersion {
				t.Errorf("expected Version handler to be called")
			}
		})
	}
}

func stringSliceEq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
