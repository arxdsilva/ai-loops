//go:build !unix

package runstore

import "os"

// No advisory locking on this OS; O_APPEND single writes are best effort.
func lock(f *os.File) error { return nil }

func unlock(f *os.File) {}
