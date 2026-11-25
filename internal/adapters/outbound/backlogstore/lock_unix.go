//go:build unix

package backlogstore

import (
	"os"
	"syscall"
)

func lock(f *os.File) error { return syscall.Flock(int(f.Fd()), syscall.LOCK_EX) }

func unlock(f *os.File) { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }

func tryLock(f *os.File) bool {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) == nil
}

// acquire takes the lane lock at p without waiting. The lock dies with the
// process; the file stays (unlinking it would race a new holder).
func acquire(p string) (func(), error) {
	f, err := os.OpenFile(p, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if !tryLock(f) {
		f.Close()
		return nil, errHeld
	}
	return func() { unlock(f); f.Close() }, nil
}

// held reports whether a live process holds the lane lock at p.
func held(p string) bool {
	f, err := os.OpenFile(p, os.O_RDWR, 0)
	if err != nil {
		return false
	}
	defer f.Close()
	if tryLock(f) {
		unlock(f)
		return false
	}
	return true
}
