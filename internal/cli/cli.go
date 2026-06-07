// Package cli is the inkwell binary's argument router. It dispatches
// between running the dashboard (`inkwell [config.yaml]`), the
// `self-update` subcommand, and the `--version` / `-v` flag.
// Keeping this in an internal package — rather than inline in
// cmd/inkwell/main.go — lets the routing logic be unit-tested
// without spawning the binary.
package cli

import (
	"fmt"
	"io"
	"strings"
)

// Options bundles the handlers and writers the router calls into.
// Handlers are injected so tests can stub them and so the wiring in
// main.go stays a one-shot composition root.
type Options struct {
	// Stdout / Stderr are where the router prints usage and errors.
	// Handlers should use these too rather than os.Stdout/os.Stderr.
	Stdout io.Writer
	Stderr io.Writer

	// RunApp is the dashboard entrypoint — called with a config path
	// for both `inkwell` (default path) and `inkwell some.yaml`.
	RunApp func(configPath string) error

	// SelfUpdate handles `inkwell self-update [...]`. The args slice
	// is everything after the subcommand name; flag parsing lives in
	// the handler.
	SelfUpdate func(args []string) error

	// Version handles `--version` / `-v`. Prints the build metadata
	// block (version, commit, build date, runtime, platform) — the
	// first token is always `inkwell vX.Y.Z` so shell scripts can
	// grep the version without a separate one-line form.
	Version func() error
}

const defaultConfigPath = "inkwell.yaml"

// Run dispatches args to the right handler and returns the process
// exit code. A successful handler returns 0; any handler error maps
// to 1 with the message on stderr. Usage errors (unknown subcommand,
// unknown top-level flag) return 2 — the convention shell scripts
// expect.
//
// Path vs. subcommand disambiguation is heuristic: a first arg that
// contains "." or "/" is treated as a config path (every realistic
// config file does); a flagless arg without those characters has to
// be a known subcommand or it's an error. This keeps
// `inkwell inkwell.yaml` and `inkwell /etc/inkwell/config.yaml`
// working while still catching typos like `inkwell self-updat`.
//
// Dispatch precedence:
//
//  1. `--version` / `-v` anywhere → Version handler.
//  2. No args → RunApp with the default config path.
//  3. First arg is `self-update` → subcommand handler.
//  4. First arg looks like a path → RunApp.
//  5. First arg starts with `-` → unknown flag → usage error.
//  6. Anything else → unknown subcommand → usage error.
func Run(args []string, opts Options) int {
	for _, a := range args {
		if a == "--version" || a == "-v" {
			return invoke(opts.Stderr, opts.Version)
		}
	}

	if len(args) == 0 {
		return invoke(opts.Stderr, func() error { return opts.RunApp(defaultConfigPath) })
	}

	head := args[0]
	rest := args[1:]

	if head == "self-update" {
		return invoke(opts.Stderr, func() error { return opts.SelfUpdate(rest) })
	}

	if looksLikePath(head) {
		return invoke(opts.Stderr, func() error { return opts.RunApp(head) })
	}

	printUsage(opts.Stderr)
	return 2
}

// looksLikePath says yes when head looks like a filesystem path
// rather than a subcommand. A leading "-" is always a flag, never a
// path. "." or "/" anywhere else means a path — every realistic
// config file matches.
func looksLikePath(head string) bool {
	if strings.HasPrefix(head, "-") {
		return false
	}
	return strings.ContainsAny(head, "./")
}

// invoke calls handler, surfaces any error on stderr, and returns the
// matching exit code. The fmt.Fprintf return is intentionally
// discarded — if writing to stderr fails the process is already in a
// bad state and there's nowhere useful to report it.
func invoke(stderr io.Writer, handler func() error) int {
	if err := handler(); err != nil {
		_, _ = fmt.Fprintf(stderr, "inkwell: %v\n", err)
		return 1
	}
	return 0
}

func printUsage(w io.Writer) {
	_, _ = fmt.Fprintln(w, "usage: inkwell [config.yaml]")
	_, _ = fmt.Fprintln(w, "       inkwell self-update [--check] [--force]")
	_, _ = fmt.Fprintln(w, "       inkwell --version | -v")
}
