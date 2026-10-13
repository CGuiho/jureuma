//go:build !windows

package cli

// releaseRunningImage is a no-op where a running executable can be deleted.
func releaseRunningImage(home, launcher string) func() { return func() {} }
