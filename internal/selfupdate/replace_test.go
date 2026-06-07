package selfupdate

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFixtureBinary creates a temp file with the given bytes and
// mode, returning its path. Cleaned up via t.Cleanup.
func writeFixtureBinary(t *testing.T, dir, name string, content []byte, mode os.FileMode) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, content, mode); err != nil {
		t.Fatalf("write fixture %s: %v", path, err)
	}
	return path
}

func TestReplaceAt_SuccessfullySwaps(t *testing.T) {
	dir := t.TempDir()
	old := []byte("OLD binary content")
	new := []byte("NEW binary content from update")
	target := writeFixtureBinary(t, dir, "inkwell", old, 0o755)
	src := writeFixtureBinary(t, t.TempDir(), "inkwell-update", new, 0o755)

	r := NewReplacer()
	if err := r.ReplaceAt(target, src); err != nil {
		t.Fatalf("ReplaceAt: %v", err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read swapped target: %v", err)
	}
	if string(got) != string(new) {
		t.Errorf("target content = %q, want %q", got, new)
	}
	st, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat swapped target: %v", err)
	}
	if st.Mode().Perm() != 0o755 {
		t.Errorf("mode = %v, want 0o755", st.Mode().Perm())
	}
}

func TestReplaceAt_CleansTempOnRenameFailure(t *testing.T) {
	dir := t.TempDir()
	target := writeFixtureBinary(t, dir, "inkwell", []byte("old"), 0o755)
	src := writeFixtureBinary(t, t.TempDir(), "inkwell-update", []byte("new"), 0o755)

	r := NewReplacer()
	r.rename = func(_, _ string) error { return errors.New("simulated rename failure") }

	err := r.ReplaceAt(target, src)
	if err == nil {
		t.Fatal("expected rename error")
	}

	// Old binary must be untouched.
	got, _ := os.ReadFile(target)
	if string(got) != "old" {
		t.Errorf("target was modified despite rename failure: %q", got)
	}

	// No orphan temp file in target dir.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if e.Name() != "inkwell" {
			t.Errorf("orphan temp file left behind: %s", e.Name())
		}
	}
}

func TestReplaceAt_ReadSrcError(t *testing.T) {
	dir := t.TempDir()
	target := writeFixtureBinary(t, dir, "inkwell", []byte("old"), 0o755)

	r := NewReplacer()
	err := r.ReplaceAt(target, filepath.Join(t.TempDir(), "nonexistent"))
	if err == nil {
		t.Fatal("expected read-src error")
	}
	if !strings.Contains(err.Error(), "read") {
		t.Errorf("error = %q, want \"read\"", err.Error())
	}
}

func TestReplaceAt_StatTargetError(t *testing.T) {
	src := writeFixtureBinary(t, t.TempDir(), "src", []byte("bin"), 0o755)

	r := NewReplacer()
	err := r.ReplaceAt(filepath.Join(t.TempDir(), "nonexistent-target"), src)
	if err == nil {
		t.Fatal("expected stat error")
	}
	if !strings.Contains(err.Error(), "stat") {
		t.Errorf("error = %q, want \"stat\"", err.Error())
	}
}

func TestReplaceAt_CreateTempError(t *testing.T) {
	dir := t.TempDir()
	target := writeFixtureBinary(t, dir, "inkwell", []byte("old"), 0o755)
	src := writeFixtureBinary(t, t.TempDir(), "src", []byte("new"), 0o755)

	r := NewReplacer()
	r.createTemp = func(string, string) (*os.File, error) {
		return nil, errors.New("simulated CreateTemp failure")
	}
	err := r.ReplaceAt(target, src)
	if err == nil {
		t.Fatal("expected create-temp error")
	}
	if !strings.Contains(err.Error(), "create temp") {
		t.Errorf("error = %q, want \"create temp\"", err.Error())
	}
}

func TestReplaceAt_WriteFileError(t *testing.T) {
	dir := t.TempDir()
	target := writeFixtureBinary(t, dir, "inkwell", []byte("old"), 0o755)
	src := writeFixtureBinary(t, t.TempDir(), "src", []byte("new"), 0o755)

	r := NewReplacer()
	r.writeFile = func(string, []byte, os.FileMode) error {
		return errors.New("simulated write failure")
	}

	err := r.ReplaceAt(target, src)
	if err == nil {
		t.Fatal("expected write error")
	}

	// Even on failure the original target must be intact.
	got, _ := os.ReadFile(target)
	if string(got) != "old" {
		t.Errorf("target = %q, want \"old\"", got)
	}
}

// TestReplace_UsesOsExecutable wires the full flow: Replace resolves
// the current binary via os.Executable + EvalSymlinks. We can't
// actually overwrite the test runner, so we stub executable to point
// at a fixture and confirm the rename lands there.
func TestReplace_UsesOsExecutable(t *testing.T) {
	dir := t.TempDir()
	target := writeFixtureBinary(t, dir, "inkwell", []byte("old"), 0o755)
	src := writeFixtureBinary(t, t.TempDir(), "src", []byte("new"), 0o755)

	r := NewReplacer()
	r.executable = func() (string, error) { return target, nil }

	if err := r.Replace(src); err != nil {
		t.Fatalf("Replace: %v", err)
	}
	got, _ := os.ReadFile(target)
	if string(got) != "new" {
		t.Errorf("target = %q, want \"new\"", got)
	}
}

func TestReplace_ExecutableError(t *testing.T) {
	r := NewReplacer()
	r.executable = func() (string, error) { return "", errors.New("simulated executable failure") }

	err := r.Replace("any.bin")
	if err == nil {
		t.Fatal("expected executable error")
	}
	if !strings.Contains(err.Error(), "resolve") {
		t.Errorf("error = %q, want \"resolve\"", err.Error())
	}
}

func TestReplace_EvalSymlinksError(t *testing.T) {
	r := NewReplacer()
	r.executable = func() (string, error) { return "/some/path/that/does/not/exist/inkwell", nil }
	// real EvalSymlinks on a nonexistent path returns error
	err := r.Replace("any.bin")
	if err == nil {
		t.Fatal("expected eval-symlinks error")
	}
}

// TestReplace_FollowsSymlink confirms EvalSymlinks resolution: a
// symlink at the executable's reported path should land the new
// binary at the symlink's target, not over the symlink itself.
func TestReplace_FollowsSymlink(t *testing.T) {
	realDir := t.TempDir()
	realBinary := writeFixtureBinary(t, realDir, "inkwell", []byte("old"), 0o755)

	linkDir := t.TempDir()
	link := filepath.Join(linkDir, "inkwell-link")
	if err := os.Symlink(realBinary, link); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	src := writeFixtureBinary(t, t.TempDir(), "src", []byte("new"), 0o755)

	r := NewReplacer()
	r.executable = func() (string, error) { return link, nil }
	if err := r.Replace(src); err != nil {
		t.Fatalf("Replace: %v", err)
	}

	// The real file (not the symlink) gets the new bytes.
	got, _ := os.ReadFile(realBinary)
	if string(got) != "new" {
		t.Errorf("realBinary content = %q, want \"new\"", got)
	}
	// And the symlink still points at the real binary.
	resolved, err := os.Readlink(link)
	if err != nil {
		t.Fatalf("Readlink: %v", err)
	}
	if resolved != realBinary {
		t.Errorf("symlink target = %q, want %q", resolved, realBinary)
	}
}
