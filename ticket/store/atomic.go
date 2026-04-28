package store

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/gofrs/flock"
)

// atomicWrite writes data to path atomically: it writes to a temp file in the
// same directory, fsyncs the temp file, renames into place, then fsyncs the
// parent directory. A crash mid-write never leaves a partial or zero-byte
// target file.
func atomicWrite(path string, data []byte) (err error) {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	// Best-effort cleanup if anything below fails before the rename succeeds.
	cleanup := func() {
		_ = os.Remove(tmpName)
	}
	defer func() {
		if err != nil {
			cleanup()
		}
	}()

	if _, werr := tmp.Write(data); werr != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp file: %w", werr)
	}
	if serr := tmp.Sync(); serr != nil {
		_ = tmp.Close()
		return fmt.Errorf("fsync temp file: %w", serr)
	}
	if cerr := tmp.Close(); cerr != nil {
		return fmt.Errorf("close temp file: %w", cerr)
	}
	if cerr := os.Chmod(tmpName, 0o644); cerr != nil { //nolint:gosec // G302: 0o644 matches the canonical ticket file mode used elsewhere in the store
		return fmt.Errorf("chmod temp file: %w", cerr)
	}
	if rerr := os.Rename(tmpName, path); rerr != nil {
		return fmt.Errorf("rename temp file: %w", rerr)
	}
	// Disable cleanup once rename has succeeded — tmpName no longer exists.
	cleanup = func() {}

	// Fsync the parent directory so the rename is durable on crash.
	d, derr := os.Open(dir) //nolint:gosec // G304: dir derived from caller-supplied target path
	if derr != nil {
		return fmt.Errorf("open parent dir: %w", derr)
	}
	defer func() { _ = d.Close() }()
	if serr := d.Sync(); serr != nil {
		return fmt.Errorf("fsync parent dir: %w", serr)
	}
	return nil
}

// withLock acquires an exclusive flock on path + ".lock", runs fn, then
// releases the lock. The lock file is created if it does not exist; it is
// kept on disk so concurrent acquirers see the same inode.
func withLock(path string, fn func() error) error {
	return withLocks([]string{path}, fn)
}

// withLocks acquires exclusive flocks on each of paths in deterministic
// (sorted) order, runs fn, then releases the locks in reverse order. Sorting
// guarantees consistent acquisition ordering across goroutines and prevents
// deadlocks when two callers lock overlapping path sets.
func withLocks(paths []string, fn func() error) error {
	ordered := make([]string, len(paths))
	copy(ordered, paths)
	sort.Strings(ordered)

	locks := make([]*flock.Flock, 0, len(ordered))
	releaseAll := func() {
		for i := len(locks) - 1; i >= 0; i-- {
			_ = locks[i].Unlock()
		}
	}
	for _, p := range ordered {
		l := flock.New(p + ".lock")
		if err := l.Lock(); err != nil {
			releaseAll()
			return fmt.Errorf("acquire lock %q: %w", p, err)
		}
		locks = append(locks, l)
	}
	defer releaseAll()
	return fn()
}
