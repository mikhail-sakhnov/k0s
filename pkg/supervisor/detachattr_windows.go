// +build windows

package supervisor
import "syscall"

func DetachAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{}
}

