//go:build !windows

package keystore

import (
	"errors"
	"os"
	"syscall"
)

// openKeyFile opens path for reading without following a final symlink.
func openKeyFile(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		if errors.Is(err, syscall.ELOOP) || errors.Is(err, syscall.EMLINK) {
			// Confirm it is a symlink rather than another ELOOP cause.
			if fi, lerr := os.Lstat(path); lerr == nil && fi.Mode()&os.ModeSymlink != 0 {
				return nil, errSymlink
			}
		}
		return nil, err
	}
	return f, nil
}

// checkKeyFile verifies the opened file is a regular file owned by the
// current user with no group/other permission bits. It returns a fixed
// reason, or "" when acceptable.
func checkKeyFile(f *os.File) string {
	fi, err := f.Stat()
	if err != nil {
		return "cannot stat key file"
	}
	if !fi.Mode().IsRegular() {
		return "key file is not a regular file"
	}
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		if int(st.Uid) != os.Geteuid() {
			return "key file is not owned by the current user"
		}
	}
	if fi.Mode().Perm()&0o077 != 0 {
		return "key file permissions are wider than 0600"
	}
	return ""
}
