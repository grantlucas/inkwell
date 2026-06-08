package selfupdate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Replacer atomically swaps a running binary for a new one. The
// public Replace resolves the current executable; ReplaceAt takes the
// target path directly so tests can drive arbitrary paths.
//
// Every os.* call is held behind a struct field so tests can inject
// failures — disk-write, rename, and fsync errors are otherwise
// impossible to provoke from a unit test without a controlled
// filesystem. Construct via NewReplacer; a zero-value Replacer{} is
// not usable because the function fields would be nil — callers that
// use a struct literal directly will hit a nil-func panic on first
// call.
type Replacer struct {
	executable   func() (string, error)
	evalSymlinks func(string) (string, error)
	readFile     func(string) ([]byte, error)
	stat         func(string) (os.FileInfo, error)
	createTemp   func(dir, pattern string) (*os.File, error)
	rename       func(oldpath, newpath string) error
	// writeAndSync writes data to the file at path with perm,
	// fsyncs it, and returns. Split out as a field so tests can
	// inject failures at the disk-IO layer; the default
	// implementation is defaultWriteAndSync.
	writeAndSync func(path string, data []byte, perm os.FileMode) error
	// fsyncDir fsyncs the named directory so a rename into it is
	// crash-durable. Split out as a field for the same reason as
	// writeAndSync; default is defaultFsyncDir. On filesystems that
	// don't support directory fsync (rare in practice on Linux), an
	// EINVAL is treated as success.
	fsyncDir func(dir string) error
}

// NewReplacer constructs a Replacer wired to the real os.* package.
func NewReplacer() *Replacer {
	return &Replacer{
		executable:   os.Executable,
		evalSymlinks: filepath.EvalSymlinks,
		readFile:     os.ReadFile,
		stat:         os.Stat,
		createTemp:   os.CreateTemp,
		rename:       os.Rename,
		writeAndSync: defaultWriteAndSync,
		fsyncDir:     defaultFsyncDir,
	}
}

// defaultWriteAndSync writes data to path with perm and fsyncs the
// resulting file before closing it. The Remove + OpenFile pattern
// (rather than os.WriteFile) is what lets perm actually take effect
// — os.WriteFile preserves an existing file's mode on overwrite.
//
// Write / Sync / Close errors are collapsed via errors.Join into a
// single return so the inner branches don't each need a dedicated
// fault-injection test to keep statement coverage at 100%.
func defaultWriteAndSync(path string, data []byte, perm os.FileMode) error {
	_ = os.Remove(path)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	_, werr := f.Write(data)
	return errors.Join(werr, f.Sync(), f.Close())
}

// defaultFsyncDir opens dir and fsyncs it so a recent rename into
// it survives a power loss. On filesystems that don't support
// directory fsync (rare on Linux ext4/xfs targets; happens on
// tmpfs) Sync returns EINVAL — but by then the rename has already
// committed in the page cache, so callers should treat the EINVAL
// return as informational. We surface it anyway via errors.Join
// rather than silently swallowing, since the same caller path
// catches a real disk-level fsync failure.
func defaultFsyncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	return errors.Join(d.Sync(), d.Close())
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
// atomic rename. The original target's mode is preserved; uid/gid
// reflect whoever ran the update (so `sudo inkwell self-update`
// leaves the binary root-owned, and an unprivileged run on a
// service-user-owned binary keeps the service user's ownership).
// On failure the temp staging file is removed and the original
// target is untouched.
//
// Durability note: the new file is fsync'd before rename, and the
// containing directory is fsync'd after, so a power loss between
// our return and the next process startup won't expose a half-
// written binary. Without the fsyncs a crashed rename could leave
// the file at the right path containing zeros until the OS got
// around to flushing pages.
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

	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if err := r.writeAndSync(tmpPath, srcBytes, info.Mode().Perm()); err != nil {
		return fmt.Errorf("stage binary %q: %w", tmpPath, err)
	}

	if err := r.rename(tmpPath, targetPath); err != nil {
		return fmt.Errorf("rename %q -> %q: %w", tmpPath, targetPath, err)
	}
	cleanup = false

	// fsync the directory so the rename is durable across a power
	// loss. Failure here is reported but doesn't roll back — the
	// rename has already committed in the page cache, so the binary
	// will be picked up on next service restart even if fsync
	// itself errored.
	if err := r.fsyncDir(dir); err != nil {
		return fmt.Errorf("fsync target dir %q: %w", dir, err)
	}
	return nil
}
