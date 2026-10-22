//go:build linux

package runner

import (
	"errors"
	"os"
	"syscall"
	"unsafe"
)

const (
	mfdCloexec      = 0x1
	mfdAllowSealing = 0x2
	fAddSeals       = 1033
	fSealSeal       = 0x1
	fSealShrink     = 0x2
	fSealGrow       = 0x4
	fSealWrite      = 0x8
)

// memfdDocument returns an anonymous in-memory file holding data, sealed
// against modification, positioned at offset 0. It never touches disk.
func memfdDocument(data []byte) (*os.File, error) {
	nr := sysMemfdCreate
	if nr < 0 {
		return nil, errors.ErrUnsupported
	}
	name, err := syscall.BytePtrFromString("zeze-document")
	if err != nil {
		return nil, err
	}
	fd, _, errno := syscall.Syscall(uintptr(nr), uintptr(unsafe.Pointer(name)), mfdCloexec|mfdAllowSealing, 0)
	if errno != 0 {
		return nil, errno
	}
	f := os.NewFile(fd, "zeze-document")
	if _, err := f.Write(data); err != nil {
		f.Close()
		return nil, err
	}
	_, _, _ = syscall.Syscall(syscall.SYS_FCNTL, fd, fAddSeals, fSealShrink|fSealGrow|fSealWrite|fSealSeal)
	if _, err := f.Seek(0, 0); err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}
