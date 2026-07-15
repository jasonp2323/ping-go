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
		logger = log.New(f, "", log.LstdFlags)
	}

	// Once detached there's no terminal to write to, so screen output is
	// meaningless in the daemon child even if -screen was left at its
	// default of true.
	effectiveScreen := cfg.screen && !daemonize.IsChild()
	if !effectiveScreen && !cfg.logEnabled {
		fmt.Fprintln(os.Stderr, "Warning: -screen=false and -log=false means output goes nowhere.")
	}

	report := func(res pinger.Result) {
		header := fmt.Sprintf("=== Ping %s at %s ===", res.Host, res.Timestamp.Format("2006-01-02 15:04:05"))

		if effectiveScreen {
			fmt.Println(header)
			fmt.Println(res.Output)
			if res.Err != nil {
				fmt.Fprintf(os.Stderr, "Error pinging %s: %v\n", res.Host, res.Err)
			}
		}
		if logger != nil {
			logger.Println(header)
			logger.Println(res.Output)
			if res.Err != nil {
				logger.Printf("Error pinging %s: %v\n", res.Host, res.Err)
			}
		}
	}

	runOne := func(ctx context.Context, host string) {
		if cfg.duration > 0 {
			output := pinger.RunForDuration(ctx, host, cfg.timeout, cfg.duration)
			report(pinger.Result{Host: host, Output: output, Timestamp: time.Now()})
			return
		}
		report(pinger.Once(ctx, host, cfg.count, cfg.timeout))
	}

	ctx := context.Background()

	if cfg.concurrent && len(cfg.hosts) > 1 {
		var wg sync.WaitGroup
		var mu sync.Mutex // serializes writes so concurrent results don't interleave
		for _, host := range cfg.hosts {
			wg.Add(1)
			go func(h string) {
				defer wg.Done()
				mu.Lock()
				defer mu.Unlock()
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
