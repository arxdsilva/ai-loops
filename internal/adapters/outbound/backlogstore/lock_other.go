//go:build !unix

package backlogstore

import (
	"errors"
	"io/fs"
	"os"
)

// No advisory locking on this OS: the state lock is best effort (run one
// supervisor), and a lane counts as live while its lock file exists. A
// lane that crashed stays running until `loops backlog finish`.
func lock(*os.File) error { return nil }

func unlock(*os.File) {}

func acquire(p string) (func(), error) {
	f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o644)
	if errors.Is(err, fs.ErrExist) {
		return nil, errHeld
	}
	if err != nil {
		return nil, err
	}
	f.Close()
	return func() { _ = os.Remove(p) }, nil
}

func held(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
