package pinger

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// fixedTime is a stable timestamp so LogLine output is deterministic.
var fixedTime = time.Date(2024, 1, 2, 15, 4, 5, 0, time.UTC)

func TestLogLineReply(t *testing.T) {
	ev := Event{
		Timestamp: fixedTime,
		Host:      "google.com",
		IP:        "142.250.80.46",
		Status:    StatusReply,
		Seq:       1,
		RTT:       12345 * time.Microsecond, // 12.345ms -> rounds to 12.3ms
		TTL:       118,
	}

	got := ev.LogLine()
	want := "2024-01-02T15:04:05Z host=google.com ip=142.250.80.46 status=reply seq=1 rtt=12.3ms ttl=118"
	if got != want {
		t.Errorf("LogLine() mismatch:\n got: %q\nwant: %q", got, want)
	}
}

func TestLogLineTimeout(t *testing.T) {
	ev := Event{
		Timestamp: fixedTime,
		Host:      "example.com",
		IP:        "93.184.216.34",
		Status:    StatusTimeout,
		Seq:       2,
	}

	got := ev.LogLine()
	// Timeouts carry no rtt/ttl fields.
	want := "2024-01-02T15:04:05Z host=example.com ip=93.184.216.34 status=timeout seq=2"
	if got != want {
		t.Errorf("LogLine() mismatch:\n got: %q\nwant: %q", got, want)
	}
}

func TestLogLineError(t *testing.T) {
	ev := Event{
		Timestamp: fixedTime,
		Host:      "nope.invalid",
		Status:    StatusError,
		Seq:       0,
		Err:       errors.New("lookup failed"),
	}

	got := ev.LogLine()
	if !strings.Contains(got, "status=error") {
		t.Errorf("expected status=error in %q", got)
	}
	if !strings.Contains(got, `err="lookup failed"`) {
		t.Errorf("expected quoted err field in %q", got)
	}
}

func TestLogLineOmitsEmptyIP(t *testing.T) {
	ev := Event{
		Timestamp: fixedTime,
		Host:      "somehost",
		Status:    StatusTimeout,
		Seq:       0,
	}

	got := ev.LogLine()
	if strings.Contains(got, "ip=") {
		t.Errorf("expected no ip= field when IP is empty, got %q", got)
	}
}

func TestLogLineTimestampIsUTC(t *testing.T) {
	// A non-UTC timestamp must still render in UTC.
	loc := time.FixedZone("UTC+5", 5*60*60)
	ev := Event{
		Timestamp: time.Date(2024, 1, 2, 20, 4, 5, 0, loc),
		Host:      "h",
		Status:    StatusTimeout,
	}

	got := ev.LogLine()
	if !strings.HasPrefix(got, "2024-01-02T15:04:05Z ") {
		t.Errorf("expected UTC-normalised timestamp prefix, got %q", got)
	}
}

func TestLogLineReplyOmitsZeroTTL(t *testing.T) {
	ev := Event{
		Timestamp: fixedTime,
		Host:      "h",
		IP:        "1.2.3.4",
		Status:    StatusReply,
		Seq:       0,
		RTT:       time.Millisecond,
		TTL:       0, // unknown TTL should be omitted
	}

	got := ev.LogLine()
	if strings.Contains(got, "ttl=") {
		t.Errorf("expected no ttl= field when TTL is zero, got %q", got)
	}
	if !strings.Contains(got, "rtt=1.0ms") {
		t.Errorf("expected rtt=1.0ms in %q", got)
	}
}
