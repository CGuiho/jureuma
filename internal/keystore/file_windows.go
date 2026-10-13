//go:build windows

package keystore

import "os"

// openKeyFile opens path for reading after refusing symlinks and reparse
// points. Mode and ownership checks are not performed on Windows.
func openKeyFile(path string) (*os.File, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if fi.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
		return nil, errSymlink
	}
	return os.Open(path)
}

// checkKeyFile verifies the opened file is a regular file.
func checkKeyFile(f *os.File) string {
	fi, err := f.Stat()
	if err != nil {
		return "cannot stat key file"
	}
	if !fi.Mode().IsRegular() {
		return "key file is not a regular file"
	}
	return ""
}
