package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/felix-nguyen/corsproxy/internal/config"
	"github.com/felix-nguyen/corsproxy/internal/netutil"
	"github.com/felix-nguyen/corsproxy/internal/proxy"
	"github.com/felix-nguyen/corsproxy/internal/session"
	"github.com/google/uuid"
)

// runServer owns the server lifecycle: listen, persist session, banner,
// serve, graceful shutdown. name is the optional user-given session label.
func runServer(cfg *config.Config, name string) error {
	// Listen before printing anything so a port conflict surfaces immediately
	// instead of after a success banner.
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		if isAddrInUse(err) {
			return fmt.Errorf("port %d is already in use — try: corsproxy --target %s --port %d",
				cfg.Port, cfg.Target, cfg.Port+1)
		}
		return err
	}

	// Persist only after Listen succeeded: servers that failed to start must
	// not pollute the resume store.
	sess := persistSession(cfg, name)

	warnIfTargetUnreachable(cfg)
	printBanner(cfg, sess.Name)

	server := &http.Server{Handler: proxy.New(cfg)}

	// Serve in the background; the main goroutine waits for Ctrl+C / SIGTERM.
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(listener) }()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
		fmt.Println("\nShutting down, waiting for in-flight requests...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		// Mimic Claude Code: after a clean exit, show how to come back.
		// Quote names containing spaces so the hint stays copy-paste-safe.
		key := resumeKey(sess)
		if strings.ContainsAny(key, " \t") {
			key = fmt.Sprintf("%q", key)
		}
		fmt.Printf("\nResume later with:  corsproxy resume %s\n", key)
		return nil
	}
}

// resumeKey prefers the human-friendly name; falls back to the uuid.
func resumeKey(sess session.Session) string {
	if sess.Name != "" {
		return sess.Name
	}
	return sess.ID
}

// persistSession records this server in the session store so it can be
// resumed later. Persistence is a convenience layer: every failure here is a
// warning, never a reason to stop the proxy from running.
func persistSession(cfg *config.Config, name string) session.Session {
	incoming := session.Session{
		ID:         uuid.NewString(),
		Name:       strings.TrimSpace(name),
		Target:     cfg.Target.String(),
		Port:       cfg.Port,
		Origin:     cfg.Origin,
		LastUsedAt: time.Now(),
	}

	path, err := session.DefaultPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠ Cannot save session: %v\n", err)
		return incoming
	}
	store := session.NewStore(path)

	sessions, err := store.Load()
	if err != nil {
		if !errors.Is(err, session.ErrCorrupt) {
			// Unreadable for another reason (permissions?) — do not risk
			// overwriting a store we could not even read.
			fmt.Fprintf(os.Stderr, "⚠ Cannot read session store: %v — this run will not be saved\n", err)
			return incoming
		}
		// A corrupt store is recoverable — warn and start fresh rather than
		// blocking the proxy over a convenience file.
		fmt.Fprintf(os.Stderr, "⚠ %v — starting a fresh session store\n", err)
		sessions = nil
	}

	// Upsert returns the session as stored: on dedupe it keeps the existing
	// ID (and name, when none was given), which the resume hint relies on.
	sessions, stored := session.Upsert(sessions, incoming)
	if err := store.Save(sessions); err != nil {
		fmt.Fprintf(os.Stderr, "⚠ Cannot save session: %v\n", err)
	}
	return stored
}

// isAddrInUse detects the "port already taken" case across platforms.
func isAddrInUse(err error) bool {
	if errors.Is(err, syscall.EADDRINUSE) {
		return true
	}
	// Windows reports a different message and errno.
	msg := err.Error()
	return strings.Contains(msg, "address already in use") ||
		strings.Contains(msg, "Only one usage of each socket address")
}

// warnIfTargetUnreachable pings the backend once. Failure is only a warning:
// the backend may simply not be started yet.
func warnIfTargetUnreachable(cfg *config.Config) {
	host := cfg.Target.Host
	if cfg.Target.Port() == "" {
		port := "80"
		if cfg.Target.Scheme == "https" {
			port = "443"
		}
		host = net.JoinHostPort(cfg.Target.Hostname(), port)
	}
	conn, err := net.DialTimeout("tcp", host, 2*time.Second)
	if err != nil {
		fmt.Printf("⚠ Cannot reach %s — proxy will still start, but remember to start your backend\n\n", cfg.Target)
		return
	}
	conn.Close()
}

func printBanner(cfg *config.Config, name string) {
	fmt.Println("\n  corsproxy is running")
	fmt.Println()
	if name != "" {
		fmt.Printf("  Name:     %s\n", name)
	}
	fmt.Printf("  Local:    http://localhost:%d\n", cfg.Port)
	if ip := netutil.LANIP(); ip != "" {
		fmt.Printf("  Network:  http://%s:%d   ← use this URL on your phone\n", ip, cfg.Port)
	}
	fmt.Printf("  Forward:  → %s\n", cfg.Target)
	fmt.Printf("  Origin:   %s\n", cfg.Origin)
	fmt.Println("\n  Press Ctrl+C to stop")
	fmt.Println()
}
