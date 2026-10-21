package markdown

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// lock serializes whole transactions across goroutines, Store instances and
// cooperating server processes. OS locks are released even if a process exits.
func (s *Store) lock(ctx context.Context) (func(), error) {
	s.mu.Lock()
	fail := func(err error) (func(), error) { s.mu.Unlock(); return nil, err }
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if err := os.MkdirAll(s.root, 0700); err != nil {
		return fail(err)
	}
	root, err := os.OpenRoot(s.root)
	if err != nil {
		return fail(err)
	}
	file, err := root.OpenFile(".waiting-mcp.lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		root.Close()
		return fail(err)
	}
	for {
		acquired, err := tryFileLock(file)
		if err != nil {
			file.Close()
			root.Close()
			return fail(err)
		}
		if acquired {
			break
		}
		select {
		case <-ctx.Done():
			file.Close()
			root.Close()
			return fail(ctx.Err())
		case <-time.After(10 * time.Millisecond):
		}
	}
	s.fs = root
	return func() { s.fs = nil; file.Close(); root.Close(); s.mu.Unlock() }, nil
}

func (s *Store) readDir(path string) ([]os.DirEntry, error) {
	dir, err := s.fs.Open(path)
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	entries, err := dir.ReadDir(-1)
	slices.SortFunc(entries, func(a, b os.DirEntry) int { return strings.Compare(a.Name(), b.Name()) })
	return entries, err
}

func (s *Store) atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := s.fs.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tempPath := filepath.Join(dir, ".tmp-"+rand.Text())
	temp, err := s.fs.OpenFile(tempPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer s.fs.Remove(tempPath)
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := s.fs.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}
