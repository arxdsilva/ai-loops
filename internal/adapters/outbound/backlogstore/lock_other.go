//go:build !unix

package backlogstore

import (
	"errors"
	"os"
)

// errNoLocks refuses backlog mode where advisory file locks that die with
// their process are not available (BACKLOG §7): lanes are separate
// processes sharing one state file, and without such a lock their writes
// would overwrite each other and a crashed lane would look alive forever.
var errNoLocks = errors.New("backlog mode needs advisory file locks (Linux, macOS or BSD); this OS has none, so loops backlog does not run here")

func lock(*os.File) error { return errNoLocks }

func unlock(*os.File) {}

func acquire(string) (*os.File, error) { return nil, errNoLocks }

func held(string) bool { return false }
