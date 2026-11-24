//go:build unix

package backlogstore

import (
	"os"
	"syscall"
)

func lock(f *os.File) error { return syscall.Flock(int(f.Fd()), syscall.LOCK_EX) }

// tryLock takes the lock without waiting; false when another holds it.
func tryLock(f *os.File) bool {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) == nil
}

func unlock(f *os.File) { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }
