//go:build windows

package runner

import "syscall"

// createNoWindow (CREATE_NO_WINDOW) prevents Windows from allocating a console
// window for child console applications. VBoxManage is a console app, so without
// this the windowed (no-console) agent would flash a cmd window on every call.
const createNoWindow = 0x08000000

func newSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
}
