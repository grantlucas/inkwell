package selfupdate

import (
	"fmt"
	"os"
	"path/filepath"
)

// exampleConfigName is the reference config bundled in every release
// tarball (.goreleaser.yaml archives.files). self-update writes a fresh
// copy next to the installed binary so users always have a current
// example of new config options. Fixed name guarantees we never touch a
// live inkwell.yaml.
const exampleConfigName = "inkwell.example.yaml"

// ExampleWriter writes the bundled inkwell.example.yaml reference next
// to the running binary. Every os.* call is held behind a struct field
// so tests can inject failures, mirroring Replacer. Construct via
// NewExampleWriter; a zero-value ExampleWriter{} is not usable because
// the function fields would be nil.
type ExampleWriter struct {
	executable   func() (string, error)
	evalSymlinks func(string) (string, error)
	writeFile    func(name string, data []byte, perm os.FileMode) error
}

// NewExampleWriter constructs an ExampleWriter wired to the real os.*
// package.
func NewExampleWriter() *ExampleWriter {
	return &ExampleWriter{
		executable:   os.Executable,
		evalSymlinks: filepath.EvalSymlinks,
		writeFile:    os.WriteFile,
	}
}

// Write drops exampleBytes next to the resolved running binary as
// inkwell.example.yaml (mode 0644), overwriting any prior reference.
// A zero-length slice is a no-op (returns nil) — a tarball predating
// the bundled example yields no bytes and there's nothing to write.
//
// The binary is resolved via os.Executable + EvalSymlinks (the same
// resolution Replacer.Replace uses) so a symlinked install writes next
// to the real binary rather than beside the link.
func (w *ExampleWriter) Write(exampleBytes []byte) error {
	if len(exampleBytes) == 0 {
		return nil
	}
	exe, err := w.executable()
	if err != nil {
		return fmt.Errorf("resolve current executable: %w", err)
	}
	resolved, err := w.evalSymlinks(exe)
	if err != nil {
		return fmt.Errorf("resolve executable symlinks for %q: %w", exe, err)
	}
	path := filepath.Join(filepath.Dir(resolved), exampleConfigName)
	if err := w.writeFile(path, exampleBytes, 0o644); err != nil {
		return fmt.Errorf("write example config %q: %w", path, err)
	}
	return nil
}
