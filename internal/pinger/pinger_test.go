package pinger

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

// fixedTime is a deterministic timestamp so LogLine output is stable.
var fixedTime = time.Date(2026, 7, 15, 17, 48, 5, 0, time.UTC)

// TestLogLine pins the exact rendered line for every combination of the
// options that affect output: each status, IP present/absent, and TTL
// present/absent. A formatting regression fails here immediately.
func TestLogLine(t *testing.T) {
	cases := []struct {
		name string
		ev   Event
		want string
	}{
		{
			name: "reply with ip and ttl",
			ev: Event{
				Timestamp: fixedTime, Host: "google.com", IP: "142.250.217.14",
				Status: StatusReply, Seq: 1, RTT: 12300 * time.Microsecond, TTL: 118,
			},
			want: "2026-07-15T17:48:05Z host=google.com ip=142.250.217.14 status=reply seq=1 rtt=12.3ms ttl=118",
		},
		{
			name: "reply without ttl omits ttl field",
			ev: Event{
				Timestamp: fixedTime, Host: "1.1.1.1", IP: "1.1.1.1",
				Status: StatusReply, Seq: 0, RTT: 400 * time.Microsecond, TTL: 0,
			},
			want: "2026-07-15T17:48:05Z host=1.1.1.1 ip=1.1.1.1 status=reply seq=0 rtt=0.4ms",
		},
		{
			name: "timeout with ip has no rtt or ttl",
			ev: Event{
				Timestamp: fixedTime, Host: "facebook.com", IP: "57.145.0.1",
				Status: StatusTimeout, Seq: 3,
			},
			want: "2026-07-15T17:48:05Z host=facebook.com ip=57.145.0.1 status=timeout seq=3",
		},
		{
			name: "timeout without ip omits ip field",
			ev: Event{
				Timestamp: fixedTime, Host: "example.com",
				Status: StatusTimeout, Seq: 2,
			},
			want: "2026-07-15T17:48:05Z host=example.com status=timeout seq=2",
		},
		{
			name: "error carries quoted err field",
			ev: Event{
				Timestamp: fixedTime, Host: "google.com", IP: "142.250.217.14",
				Status: StatusError, Seq: 0,
				Err: errors.New("socket: the requested protocol has not been configured into the system"),
			},
			want: `2026-07-15T17:48:05Z host=google.com ip=142.250.217.14 status=error seq=0 err="socket: the requested protocol has not been configured into the system"`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.ev.LogLine(); got != tc.want {
				t.Errorf("LogLine()\n got: %s\nwant: %s", got, tc.want)
			}
		})
	}
}

// TestLogLineTimestampAlwaysUTC guards against a local-time regression: a
// non-UTC input must still render with a trailing Z.
func TestLogLineTimestampAlwaysUTC(t *testing.T) {
	loc := time.FixedZone("UTC+5", 5*3600)
	ev := Event{Timestamp: time.Date(2026, 7, 15, 22, 48, 5, 0, loc), Host: "h", Status: StatusTimeout}
	got := ev.LogLine()
	want := "2026-07-15T17:48:05Z host=h status=timeout seq=0"
	if got != want {
		t.Errorf("LogLine() did not normalize to UTC\n got: %s\nwant: %s", got, want)
	}
}

// TestLoopbackReply exercises the real ICMP path against 127.0.0.1 and
// asserts we get a reply — never a socket/protocol error. This is the
// check that catches platform regressions like Windows rejecting
// unprivileged UDP ping ("the requested protocol has not been configured
// into the system"). It only runs when PINGGO_INTEGRATION is set, so it
// doesn't fail on dev machines that lack ping privileges; CI sets the env
// and configures the necessary privileges per OS.
func TestLoopbackReply(t *testing.T) {
	if os.Getenv("PINGGO_INTEGRATION") == "" {
		t.Skip("set PINGGO_INTEGRATION=1 to run the loopback ICMP integration test")
	}

	var got []Event
	Once(context.Background(), "127.0.0.1", 2, 3*time.Second, false, func(ev Event) {
		got = append(got, ev)
	})

	if len(got) != 2 {
		t.Fatalf("expected 2 events, got %d: %+v", len(got), got)
	}
	for _, ev := range got {
		if ev.Status == StatusError {
			t.Fatalf("loopback ping returned an error status (likely a platform/privilege regression): %s", ev.LogLine())
		}
		if ev.Status != StatusReply {
			t.Errorf("expected loopback to reply, got: %s", ev.LogLine())
		}
	}
}
