//go:build unix

package config

import (
	"os"
	"syscall"
)

// osConfigFileOwner reports whether path is owned by the current effective user.
// The second return is the owner uid for diagnostics (-1 if it cannot be read).
func osConfigFileOwner(path string) (owned bool, ownerUID int, err error) {
	info, err := os.Lstat(path)
	if err != nil {
		return false, -1, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return true, -1, nil
	}
	uid := int(stat.Uid)
	return uid == os.Geteuid(), uid, nil
}
