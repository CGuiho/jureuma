//go:build linux && !amd64 && !386 && !arm && !arm64 && !riscv64 && !loong64

package runner

// Unknown syscall number on this architecture: use the file fallback.
const sysMemfdCreate = -1
