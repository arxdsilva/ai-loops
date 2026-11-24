//go:build !unix

package backlogstore

import "os"

// No advisory locking on this OS: one supervisor at a time, and a lane
// counts as live while its lock file exists (a crashed lane stays running
// until `loops backlog finish`).
func lock(*os.File) error { return nil }

func tryLock(f *os.File) bool {
	fi, err := f.Stat()
	return err == nil && fi.Size() == 0
}

func unlock(*os.File) {}
