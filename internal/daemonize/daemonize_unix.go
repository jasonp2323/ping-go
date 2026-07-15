//go:build !windows

package daemonize

import "syscall"

// newDetachedSysProcAttr returns the Unix-specific attributes needed to
// start a new session so the child has no controlling terminal and won't
// receive SIGHUP when the parent shell closes.
func newDetachedSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setsid: true,
	}
}
