// Package daemonize lets the current binary re-exec itself as a detached
// background process, so the tool doesn't need nohup, systemd, or
// Start-Process wrapping to survive the parent shell closing.
package daemonize

import (
	"fmt"
	"os"
	"os/exec"
)

// EnvMarker is set in the child process's environment so it knows not to
// daemonize again (preventing infinite re-exec).
const EnvMarker = "PING_GO_DAEMON_CHILD=1"

// IsChild reports whether the current process is already the detached child.
func IsChild() bool {
	for _, e := range os.Environ() {
		if e == EnvMarker {
			return true
		}
	}
	return false
}

// sysProcAttrForDetach is implemented per-platform (see daemonize_windows.go
// and daemonize_unix.go) since the fields on syscall.SysProcAttr differ
// between Windows and Unix.
//
// Declared here as a function value so this file has no build tags itself.
var sysProcAttrForDetach = newDetachedSysProcAttr

// Start re-execs the current binary with the same arguments (minus -daemon),
// fully detached from the current console/terminal, and returns the PID of
// the new process. The caller is expected to exit immediately afterward.
func Start(logPath string) (pid int, err error) {
	exePath, err := os.Executable()
	if err != nil {
		return 0, fmt.Errorf("resolving executable path: %w", err)
	}

	var childArgs []string
	for _, a := range os.Args[1:] {
		if a != "-daemon" && a != "--daemon" {
			childArgs = append(childArgs, a)
		}
	}

	cmd := exec.Command(exePath, childArgs...)
	cmd.Env = append(os.Environ(), EnvMarker)

	var outFile *os.File
	if logPath != "" {
		outFile, err = os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	} else {
		outFile, err = os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	}
	if err != nil {
		return 0, fmt.Errorf("opening daemon output target: %w", err)
	}

	cmd.Stdin = nil
	cmd.Stdout = outFile
	cmd.Stderr = outFile
	cmd.SysProcAttr = sysProcAttrForDetach()

	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("starting daemon process: %w", err)
	}

	return cmd.Process.Pid, nil
}
