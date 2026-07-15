//go:build windows

package daemonize

import "syscall"

const (
	createNewProcessGroup = 0x00000200
	detachedProcess       = 0x00000008
	createNoWindow        = 0x08000000
)

// newDetachedSysProcAttr returns the Windows-specific attributes needed to
// spawn a process with no console window that is not tied to the parent's
// console session (so closing PowerShell/cmd won't kill it).
func newDetachedSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags: createNewProcessGroup | detachedProcess | createNoWindow,
	}
}
