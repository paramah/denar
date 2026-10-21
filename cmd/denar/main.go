// Command denar is the composition root of the server.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	// Time zones of households are checked against the IANA database; the
	// embedded copy works in images without one.
	_ "time/tzdata"

	"git.eiac.dev/tools/denar/internal/adapter/sqlite"
	"git.eiac.dev/tools/denar/internal/platform/config"
	"git.eiac.dev/tools/denar/web"
)

var version = "dev"

const usage = `usage: denar <command>

commands:
  serve          run the HTTP server
  migrate        bring the database schema up to date
  token create   create a bot token (see denar token create -h)
  demo           fill the database with a demo household (see denar demo -h)
  version        print the version`

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "denar:", err)
		os.Exit(1)
	}
}

// run executes one command. Logs go to stdout as JSON (to stderr for the
// token command, whose stdout is the token); ctx ends serve.
func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return errors.New("missing command\n" + usage)
	}
	switch args[0] {
	case "serve", "migrate", "token", "demo":
	case "version":
		fmt.Fprintln(stdout, version)
		return nil
	default:
		return fmt.Errorf("unknown command %q\n%s", args[0], usage)
	}

	cfg, err := config.FromEnv()
	if err != nil {
		return fmt.Errorf("configuration:\n%w", err)
	}
	switch args[0] {
	case "migrate":
		return migrate(ctx, cfg, slog.New(slog.NewJSONHandler(stdout, nil)))
	case "token":
		return tokenCommand(ctx, cfg, args[1:], stdout, stderr)
	case "demo":
		return demoCommand(ctx, cfg, args[1:], stdout, stderr)
	}
	return serve(ctx, cfg, slog.New(slog.NewJSONHandler(stdout, nil)), nil)
}

func migrate(ctx context.Context, cfg config.Config, log *slog.Logger) error {
	db, err := sqlite.Open(cfg.DatabasePath)
	if err != nil {
		return err
	}
	if err := sqlite.Migrate(ctx, db, sqlite.Migrations()); err != nil {
		db.Close()
		return err
	}
	version, _, err := sqlite.SchemaVersion(ctx, db, sqlite.Migrations())
	if err != nil {
		db.Close()
		return err
	}
	log.Info("database schema is up to date", "database", cfg.DatabasePath, "version", version)
	return db.Close()
}

// serve runs the HTTP server until ctx is done, then lets requests finish
// and closes the database. listening, when not nil, is told the address.
func serve(ctx context.Context, cfg config.Config, log *slog.Logger, listening func(net.Addr)) error {
	a, err := newApp(ctx, cfg, log, web.Dist())
	if err != nil {
		return err
	}
	defer a.Close()

	listener, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return err
	}
	srv := &http.Server{
		Handler:           a.handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(listener) }()
	log.Info("listening", "addr", listener.Addr().String(), "config", cfg)
	if listening != nil {
		listening(listener.Addr())
	}

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}

	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	return a.Close()
}
