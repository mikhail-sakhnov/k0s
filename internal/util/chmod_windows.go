package util

import "os"
import "github.com/hectane/go-acl"

// Chmod operates file permisssions
func Chmod(name string, mode os.FileMode) error {
	return acl.Chmod(name, mode)
}
