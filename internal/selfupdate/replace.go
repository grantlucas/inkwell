package selfupdate

import (
	"fmt"
	"os"
	"path/filepath"
)

// Replacer atomically swaps a running binary for a new one. The
// public Replace resolves the current executable; ReplaceAt takes the
// target path directly so tests can drive arbitrary paths.
//
// Every os.* call is held behind a struct field so tests can inject
// failures — disk-write and rename errors are otherwise impossible
// to provoke from a unit test without a controlled filesystem.
type Replacer struct {
	executable   func() (string, error)
	evalSymlinks func(string) (string, error)
	readFile     func(string) ([]byte, error)
	stat         func(string) (os.FileInfo, error)
	createTemp   func(dir, pattern string) (*os.File, error)
	writeFile    func(name string, data []byte, perm os.FileMode) error
	rename       func(oldpath, newpath string) error
}

// NewReplacer constructs a Replacer wired to the real os.* package.
func NewReplacer() *Replacer {
	return &Replacer{
		executable:   os.Executable,
		evalSymlinks: filepath.EvalSymlinks,
		readFile:     os.ReadFile,
		stat:         os.Stat,
		createTemp:   os.CreateTemp,
		writeFile:    os.WriteFile,
		rename:       os.Rename,
	}
}

// Replace atomically replaces the currently-running binary with the
// contents of srcPath. Resolves the running executable via
// os.Executable + filepath.EvalSymlinks so a symlinked install
// (e.g. /usr/local/bin/inkwell → /opt/inkwell/bin/inkwell) updates
// the real file rather than overwriting the link.
func (r *Replacer) Replace(srcPath string) error {
	exe, err := r.executable()
	if err != nil {
		return fmt.Errorf("resolve current executable: %w", err)
	}
	resolved, err := r.evalSymlinks(exe)
	if err != nil {
		return fmt.Errorf("resolve executable symlinks for %q: %w", exe, err)
	}
	return r.ReplaceAt(resolved, srcPath)
}

// ReplaceAt swaps the file at targetPath with the bytes from
// srcPath. The new file is staged at a sibling temp path in
// targetPath's directory so the final os.Rename is a same-filesystem
// atomic rename. The original mode is preserved (chown for uid/gid
// is best-effort and not enforced — the systemd install runs as a
// known user, so the rename inherits the right owner via the
// directory). On failure the temp staging file is removed and the
// original target is untouched.
func (r *Replacer) ReplaceAt(targetPath, srcPath string) error {
	srcBytes, err := r.readFile(srcPath)
	if err != nil {
		return fmt.Errorf("read source binary %q: %w", srcPath, err)
	}

	info, err := r.stat(targetPath)
	if err != nil {
		return fmt.Errorf("stat target %q: %w", targetPath, err)
	}

	dir := filepath.Dir(targetPath)
	tmp, err := r.createTemp(dir, ".inkwell-update-*")
	if err != nil {
		return fmt.Errorf("create temp in target dir %q: %w", dir, err)
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()
	// CreateTemp wrote a 0600 placeholder; remove it so writeFile
	// creates fresh with the target's mode.
	_ = os.Remove(tmpPath)

	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if err := r.writeFile(tmpPath, srcBytes, info.Mode().Perm()); err != nil {
		return fmt.Errorf("write staged binary %q: %w", tmpPath, err)
	}

	if err := r.rename(tmpPath, targetPath); err != nil {
		return fmt.Errorf("rename %q -> %q: %w", tmpPath, targetPath, err)
	}
	cleanup = false
	return nil
}
