// +build !windows

package util

import "os"

// Chmod operates file permisssions
func Chmod(name string, mode os.FileMode) error {
	return os.Chmod(name, mode)
}
