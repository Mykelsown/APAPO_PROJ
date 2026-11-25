// Command api runs the Apapo backend HTTP server.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Mykelsown/apapo-backend/internal/config"
	"github.com/Mykelsown/apapo-backend/internal/server"
)

// buildVersion is set with -ldflags "-X main.buildVersion=...".
var buildVersion = ""

func main() {
	healthcheck := flag.Bool("healthcheck", false, "call /healthz on the local server and exit 0 if healthy (used by Docker HEALTHCHECK)")
	flag.Parse()
	log.SetFlags(log.LstdFlags | log.LUTC)

	cfg, err := config.Load(os.LookupEnv, buildVersion)
	if err != nil {
		log.Fatalf("config error: %v", err)
	}
	if *healthcheck {
		os.Exit(runHealthcheck(cfg.Addr))
	}

	log.Printf("apapo-api starting: %s", cfg.Summary())
	if cfg.SecretGenerated {
		log.Printf("warning: APP_SECRET is not set, a random one is used; set it so tokens survive restarts")
	}

	api := server.New(cfg)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	// Background passes: reversal retries and the Split-Receive scan, every 10 seconds.
	api.World().StartWorkers(ctx, 10*time.Second)

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           api.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("listening on %s", cfg.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		if err != nil {
			log.Fatalf("server error: %v", err)
		}
	case <-ctx.Done():
		log.Printf("shutdown signal received, draining requests")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
	log.Printf("stopped")
}

// runHealthcheck calls /healthz on the local listener. It returns a process exit code.
func runHealthcheck(addr string) int {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "bad address:", err)
		return 1
	}
	if host == "" || host == "0.0.0.0" || host == "::" || strings.HasPrefix(host, "[") {
		host = "127.0.0.1"
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://" + net.JoinHostPort(host, port) + "/healthz")
	if err != nil {
		fmt.Fprintln(os.Stderr, "unhealthy:", err)
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "unhealthy: status", resp.StatusCode)
		return 1
	}
	return 0
}
