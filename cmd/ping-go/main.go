// Command ping-go pings one or more hosts, optionally concurrently,
// optionally for a fixed duration, with output to screen and/or a log
// file, and can detach itself into the background without relying on
// OS-specific tooling like nohup or Start-Process.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/yourusername/ping-go/internal/daemonize"
	"github.com/yourusername/ping-go/internal/pinger"
)

type config struct {
	hosts      []string
	screen     bool
	logEnabled bool
	logPath    string
	concurrent bool
	count      int
	timeout    time.Duration
	duration   time.Duration
	daemon     bool
	privileged bool
}

func parseFlags() config {
	hostsFlag := flag.String("hosts", "", "Comma-separated list of hosts to ping (e.g. google.com,8.8.8.8)")
	screen := flag.Bool("screen", true, "Print output to screen")
	logEnabled := flag.Bool("log", false, "Enable logging to file")
	logPath := flag.String("logfile", "ping.log", "Path to log file (used if -log is set)")
	concurrent := flag.Bool("concurrent", false, "Ping all hosts concurrently instead of one at a time")
	count := flag.Int("count", 4, "Number of echoes to send per host (ignored if -duration is set)")
	timeout := flag.Duration("timeout", 5*time.Second, "Timeout per ping invocation, e.g. 3s, 500ms")
	duration := flag.Duration("duration", 0, "Keep pinging each host for this long, e.g. 30s, 2m (overrides -count)")
	daemon := flag.Bool("daemon", false, "Detach and run in the background")
	privileged := flag.Bool("privileged", false, "Use raw ICMP sockets (needs root/CAP_NET_RAW on Linux/macOS); default is unprivileged UDP")
	flag.Parse()

	if *hostsFlag == "" {
		fmt.Fprintln(os.Stderr, "Error: at least one host is required. Use -hosts=host1,host2")
		flag.Usage()
		os.Exit(1)
	}

	var hosts []string
	for _, h := range strings.Split(*hostsFlag, ",") {
		if h = strings.TrimSpace(h); h != "" {
			hosts = append(hosts, h)
		}
	}

	return config{
		hosts:      hosts,
		screen:     *screen,
		logEnabled: *logEnabled,
		logPath:    *logPath,
		concurrent: *concurrent,
		count:      *count,
		timeout:    *timeout,
		duration:   *duration,
		daemon:     *daemon,
		privileged: *privileged,
	}
}

func main() {
	cfg := parseFlags()

	// Re-exec as a detached background process, then let the parent exit.
	if cfg.daemon && !daemonize.IsChild() {
		lp := ""
		if cfg.logEnabled {
			lp = cfg.logPath
		}
		pid, err := daemonize.Start(lp)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error starting daemon: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Started in background, PID %d\n", pid)
		if lp != "" {
			fmt.Printf("Output redirected to %s\n", lp)
		} else {
			fmt.Println("No -logfile set; daemon output is discarded. Add -log=true -logfile=<path> to capture it.")
		}
		return
	}

	var logger *log.Logger
	if cfg.logEnabled {
		f, err := os.OpenFile(cfg.logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening log file: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		// Each line already carries its own RFC3339 timestamp, so the
		// logger adds no prefix or flags of its own.
		logger = log.New(f, "", 0)
	}

	// Once detached there's no terminal to write to, so screen output is
	// meaningless in the daemon child even if -screen was left at its
	// default of true.
	effectiveScreen := cfg.screen && !daemonize.IsChild()
	if !effectiveScreen && !cfg.logEnabled {
		fmt.Fprintln(os.Stderr, "Warning: -screen=false and -log=false means output goes nowhere.")
	}

	// emit writes a single ping event as one line to the enabled sinks.
	// The mutex keeps individual lines from interleaving when hosts are
	// pinged concurrently; each line stands alone (it names its own host
	// and timestamp), so streaming interleaved lines is fine for a log.
	var mu sync.Mutex
	emit := func(ev pinger.Event) {
		line := ev.LogLine()
		mu.Lock()
		defer mu.Unlock()
		if effectiveScreen {
			fmt.Println(line)
		}
		if logger != nil {
			logger.Println(line)
		}
	}

	runOne := func(ctx context.Context, host string) {
		if cfg.duration > 0 {
			pinger.RunForDuration(ctx, host, cfg.timeout, cfg.duration, cfg.privileged, emit)
			return
		}
		pinger.Once(ctx, host, cfg.count, cfg.timeout, cfg.privileged, emit)
	}

	ctx := context.Background()

	if cfg.concurrent && len(cfg.hosts) > 1 {
		var wg sync.WaitGroup
		for _, host := range cfg.hosts {
			wg.Add(1)
			go func(h string) {
				defer wg.Done()
				runOne(ctx, h)
			}(host)
		}
		wg.Wait()
	} else {
		for _, host := range cfg.hosts {
			runOne(ctx, host)
		}
	}
}
