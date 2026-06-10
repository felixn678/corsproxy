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
)

// runServer owns the server lifecycle: listen, banner, serve, graceful shutdown.
func runServer(cfg *config.Config) error {
	// Listen before printing anything so a port conflict surfaces immediately
	// instead of after a success banner.
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		if isAddrInUse(err) {
			return fmt.Errorf("port %d đang được sử dụng — thử: corsproxy --target %s --port %d",
				cfg.Port, cfg.Target, cfg.Port+1)
		}
		return err
	}

	warnIfTargetUnreachable(cfg)
	printBanner(cfg)

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
		fmt.Println("\nĐang dừng, đợi các request đang chạy...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	}
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
		fmt.Printf("⚠ Chưa kết nối được %s — proxy vẫn chạy, nhớ bật backend\n\n", cfg.Target)
		return
	}
	conn.Close()
}

func printBanner(cfg *config.Config) {
	fmt.Println("\n  corsproxy đang chạy")
	fmt.Println()
	fmt.Printf("  Local:    http://localhost:%d\n", cfg.Port)
	if ip := netutil.LANIP(); ip != "" {
		fmt.Printf("  Network:  http://%s:%d   ← dùng URL này trên phone\n", ip, cfg.Port)
	}
	fmt.Printf("  Forward:  → %s\n", cfg.Target)
	fmt.Printf("  Origin:   %s\n", cfg.Origin)
	fmt.Println("\n  Ctrl+C để dừng")
	fmt.Println()
}
