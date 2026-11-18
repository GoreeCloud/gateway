// GoreeCloud Gateway combines Caddy's upstream Go engine with an original local console.
package main

import (
	"context"
	"embed"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/GoreeCloud/gateway/internal/gateway"
	_ "github.com/caddy-dns/porkbun"
	caddycmd "github.com/caddyserver/caddy/v2/cmd"
	_ "github.com/caddyserver/caddy/v2/modules/standard"
)

//go:embed web/dist
var builtWeb embed.FS

func main() {
	if len(os.Args) > 1 && os.Args[1] == "serve" {
		if err := serve(os.Args[2:]); err != nil {
			log.Printf("gateway: %v", err)
			os.Exit(1)
		}
		return
	}
	// "run", "adapt", "validate", and the other upstream Caddy commands remain available.
	caddycmd.Main()
}

func serve(args []string) error {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	listen := flags.String("listen", "127.0.0.1:8787", "loopback-only management address")
	sitesPath := flags.String("sites", "./gateway-sites.json", "managed site state file")
	allowWrite := flags.Bool("allow-write", false, "enable authenticated site changes")
	stateDir := flags.String("state-dir", "", "private Gateway-managed certificate and autosave directory")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected arguments")
	}
	host, _, err := net.SplitHostPort(*listen)
	if err != nil {
		return fmt.Errorf("invalid console listen address: %w", err)
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return errors.New("the management console must bind to an IP loopback address")
	}
	token := os.Getenv("GATEWAY_ADMIN_TOKEN")
	if len(token) < 24 {
		return errors.New("set GATEWAY_ADMIN_TOKEN to a random secret of at least 24 characters")
	}

	managedDir := *stateDir
	if managedDir == "" {
		managedDir = filepath.Join(filepath.Dir(*sitesPath), ".gateway-state")
	}
	if err := gateway.PrepareManagedStorage(managedDir); err != nil {
		return fmt.Errorf("unsafe managed runtime storage: %w", err)
	}
	manager, err := gateway.Open(*sitesPath, nil)
	if err != nil {
		return err
	}
	if err := manager.Start(); err != nil {
		return fmt.Errorf("could not load managed ingress: %w", err)
	}
	dist, err := fs.Sub(builtWeb, "web/dist")
	if err != nil {
		return err
	}
	handler := gateway.Handler(manager, dist, token, *allowWrite)
	server := &http.Server{
		Addr: *listen, Handler: handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}
	listener, err := net.Listen("tcp", *listen)
	if err != nil {
		return fmt.Errorf("cannot bind management listener: %w", err)
	}
	log.Printf("GoreeCloud Gateway console on http://%s (write=%t)", listener.Addr(), *allowWrite)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	err = server.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
