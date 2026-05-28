package lockfile

import (
	"fmt"
	"os"
	"path/filepath"
)

// Lock represents an exclusive backup lock.
type Lock struct {
	path string
	f    *os.File
}

// Acquire creates an exclusive lock file.
func Acquire(path string) (*Lock, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && !os.IsNotExist(err) {
		// lock file may be in temp dir without parent creation need
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return nil, fmt.Errorf("backup already in progress (lock: %s)", path)
		}
		return nil, err
	}
	_, _ = f.WriteString(fmt.Sprintf("%d\n", os.Getpid()))
	return &Lock{path: path, f: f}, nil
}

func (l *Lock) Release() error {
	if l.f != nil {
		l.f.Close()
	}
	return os.Remove(l.path)
}
