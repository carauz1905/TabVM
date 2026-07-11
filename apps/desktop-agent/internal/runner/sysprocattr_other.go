//go:build !windows

package runner

import "syscall"

func newSysProcAttr() *syscall.SysProcAttr {
	return nil
}
