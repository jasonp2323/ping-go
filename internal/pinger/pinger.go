// Package pinger wraps the OS-native ping command so the rest of the
// program doesn't need to care about platform-specific flags.
package pinger

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// Result holds the outcome of pinging a single host.
type Result struct {
	Host      string
	Output    string
	Err       error
	Timestamp time.Time
}

// Once runs a single ping invocation (with count echoes) against host,
// bounded by the given timeout. It shells out to the OS "ping" binary
// rather than using raw ICMP sockets, so it needs no special privileges.
func Once(ctx context.Context, host string, count int, timeout time.Duration) Result {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "ping", "-n", fmt.Sprintf("%d", count), host)
	} else {
		cmd = exec.CommandContext(ctx, "ping", "-c", fmt.Sprintf("%d", count), host)
	}

	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		err = fmt.Errorf("ping timed out after %s", timeout)
	}

	return Result{
		Host:      host,
		Output:    string(out),
		Err:       err,
		Timestamp: time.Now(),
	}
}

// RunForDuration repeatedly pings host (one echo per second) until
// totalDuration has elapsed, respecting perPingTimeout on each attempt.
// It returns the combined output of every attempt.
func RunForDuration(ctx context.Context, host string, perPingTimeout, totalDuration time.Duration) string {
	var sb strings.Builder
	deadline := time.Now().Add(totalDuration)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return sb.String()
		default:
		}

		res := Once(ctx, host, 1, perPingTimeout)
		sb.WriteString(res.Output)
		if res.Err != nil {
			sb.WriteString(fmt.Sprintf("Error: %v\n", res.Err))
		}
		time.Sleep(1 * time.Second)
	}
	return sb.String()
}
