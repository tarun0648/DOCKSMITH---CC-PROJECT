//go:build !linux

package builder

import "fmt"

// RunIsolated is not supported on non-Linux platforms.
func RunIsolated(rootDir string, command string, envVars []string, workdir string) error {
	return fmt.Errorf("container isolation requires Linux; please use a Linux VM (WSL2, VirtualBox, etc.)")
}

// ChrootChild is a no-op on non-Linux.
func ChrootChild() {
	panic("ChrootChild called on non-Linux platform")
}
