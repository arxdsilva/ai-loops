//go:build unix

package hwprobe

import "syscall"

func diskFree(dir string) (uint64, error) {
	if dir == "" {
		dir = "."
	}
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return 0, err
	}
	return uint64(st.Bavail) * uint64(st.Bsize), nil //nolint:unconvert // field types differ per OS
}
