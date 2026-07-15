// Package pinger sends ICMP echoes with a native Go library (pro-bing)
// rather than shelling out to the OS "ping" binary, so it produces a
// clean, structured event per echo instead of platform-specific text.
package pinger

import (
	"context"
	"fmt"
	"net"
	"runtime"
	"strings"
	"time"

	probing "github.com/prometheus-community/pro-bing"
)

// Status describes the outcome of a single ping attempt.
type Status string

const (
	StatusReply   Status = "reply"   // an echo reply came back
	StatusTimeout Status = "timeout" // no reply within the timeout
	StatusError   Status = "error"   // setup/resolution/socket failure
)

// Event is the result of a single ping echo. One Event maps to one line
// of output, so every ping carries its own timestamp.
type Event struct {
	Timestamp time.Time
	Host      string // host as given on the command line
	IP        string // resolved IP, when known
	Status    Status
	Seq       int           // sequence number within this host's run
	RTT       time.Duration // round-trip time (StatusReply only)
	TTL       int           // reply TTL (StatusReply only)
	Err       error         // populated for StatusError
}

// LogLine renders the event as a single logfmt-style line:
//
//	2006-01-02T15:04:05Z host=google.com ip=142.250.80.46 status=reply seq=1 rtt=12.3ms ttl=118
//
// The leading field is a sortable RFC3339 UTC timestamp; every other
// field is key=value so the output greps and parses cleanly.
func (e Event) LogLine() string {
	var b strings.Builder
	b.WriteString(e.Timestamp.UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, " host=%s", e.Host)
	if e.IP != "" {
		fmt.Fprintf(&b, " ip=%s", e.IP)
	}
	fmt.Fprintf(&b, " status=%s seq=%d", e.Status, e.Seq)
	if e.Status == StatusReply {
		fmt.Fprintf(&b, " rtt=%.1fms", float64(e.RTT)/float64(time.Millisecond))
		if e.TTL > 0 {
			fmt.Fprintf(&b, " ttl=%d", e.TTL)
		}
	}
	if e.Err != nil {
		fmt.Fprintf(&b, " err=%q", e.Err.Error())
	}
	return b.String()
}

// pingOnce sends a single ICMP echo to host and reports the outcome.
// ip, when non-empty, is a pre-resolved address used to skip a fresh DNS
// lookup on every echo (and to still label timeouts with the target IP).
func pingOnce(ctx context.Context, host, ip string, seq int, timeout time.Duration, privileged bool) Event {
	ev := Event{Timestamp: time.Now(), Host: host, IP: ip, Seq: seq, Status: StatusTimeout}

	p, err := probing.NewPinger(host)
	if err != nil {
		ev.Status = StatusError
		ev.Err = err
		return ev
	}
	if ip != "" {
		p.SetIPAddr(&net.IPAddr{IP: net.ParseIP(ip)})
	}
	// Windows has no unprivileged UDP-based ping; attempting it fails with
	// "the requested protocol has not been configured into the system", so
	// always use raw ICMP there (which needs no admin rights on Windows).
	if runtime.GOOS == "windows" {
		privileged = true
	}
	p.SetPrivileged(privileged)
	p.Count = 1
	p.Timeout = timeout

	p.OnRecv = func(pkt *probing.Packet) {
		ev.Timestamp = time.Now()
		ev.Status = StatusReply
		ev.IP = pkt.IPAddr.String()
		ev.RTT = pkt.Rtt
		ev.TTL = pkt.TTL
	}

	if err := p.RunWithContext(ctx); err != nil {
		if ev.Status != StatusReply {
			ev.Status = StatusError
			ev.Err = err
		}
	}
	return ev
}

// resolve looks the host up once so repeated echoes reuse the same IP and
// timeouts can still be labelled with a target address. A resolution
// failure is non-fatal: pingOnce will retry resolution itself.
func resolve(host string) string {
	p, err := probing.NewPinger(host)
	if err != nil {
		return ""
	}
	if err := p.Resolve(); err != nil {
		return ""
	}
	if addr := p.IPAddr(); addr != nil {
		return addr.String()
	}
	return ""
}

// Once sends count echoes to host, calling emit for each attempt.
func Once(ctx context.Context, host string, count int, timeout time.Duration, privileged bool, emit func(Event)) {
	ip := resolve(host)
	for seq := 0; seq < count; seq++ {
		if ctx.Err() != nil {
			return
		}
		emit(pingOnce(ctx, host, ip, seq, timeout, privileged))
	}
}

// RunForDuration pings host once per second until totalDuration elapses,
// bounding each echo by perPingTimeout and calling emit for every attempt.
func RunForDuration(ctx context.Context, host string, perPingTimeout, totalDuration time.Duration, privileged bool, emit func(Event)) {
	ip := resolve(host)
	deadline := time.Now().Add(totalDuration)
	for seq := 0; time.Now().Before(deadline); seq++ {
		if ctx.Err() != nil {
			return
		}
		emit(pingOnce(ctx, host, ip, seq, perPingTimeout, privileged))
		time.Sleep(1 * time.Second)
	}
}
