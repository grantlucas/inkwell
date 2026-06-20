package selfupdate

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestExampleWriter_Write_Success confirms the example bytes land next
// to the resolved running binary, named inkwell.example.yaml, mode 0644.
func TestExampleWriter_Write_Success(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "inkwell")
	want := []byte("# example config\n")

	w := &ExampleWriter{
		executable:   func() (string, error) { return exe, nil },
		evalSymlinks: func(p string) (string, error) { return p, nil },
		writeFile:    os.WriteFile,
	}

	if err := w.Write(want); err != nil {
		t.Fatalf("Write: %v", err)
	}

	path := filepath.Join(dir, "inkwell.example.yaml")
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written example: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("example bytes = %q, want %q", got, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat example: %v", err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Errorf("example mode = %v, want 0644", info.Mode().Perm())
	}
}

func TestExampleWriter_Write_ErrorPaths(t *testing.T) {
	sentinel := errors.New("boom")
	tests := []struct {
		label string
		w     *ExampleWriter
		bytes []byte
		// wantFile: a path that must NOT exist after Write, or "" to skip.
		wantErr bool
	}{
		{
			label: "empty bytes is a no-op",
			w: &ExampleWriter{
				executable:   func() (string, error) { t.Fatal("executable must not be called"); return "", nil },
				evalSymlinks: func(p string) (string, error) { return p, nil },
				writeFile:    os.WriteFile,
			},
			bytes:   nil,
			wantErr: false,
		},
		{
			label: "executable error",
			w: &ExampleWriter{
				executable:   func() (string, error) { return "", sentinel },
				evalSymlinks: func(p string) (string, error) { return p, nil },
				writeFile:    os.WriteFile,
			},
			bytes:   []byte("x"),
			wantErr: true,
		},
		{
			label: "evalSymlinks error",
			w: &ExampleWriter{
				executable:   func() (string, error) { return "/some/inkwell", nil },
				evalSymlinks: func(p string) (string, error) { return "", sentinel },
				writeFile:    os.WriteFile,
			},
			bytes:   []byte("x"),
			wantErr: true,
		},
		{
			label: "writeFile error",
			w: &ExampleWriter{
				executable:   func() (string, error) { return "/some/inkwell", nil },
				evalSymlinks: func(p string) (string, error) { return p, nil },
				writeFile:    func(string, []byte, os.FileMode) error { return sentinel },
			},
			bytes:   []byte("x"),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.label, func(t *testing.T) {
			err := tc.w.Write(tc.bytes)
			if tc.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
