package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"sea-codex-server/internal/bootstrap"
	"sea-codex-server/pkg/config"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, output io.Writer) (runErr error) {
	flags := flag.NewFlagSet("api", flag.ContinueOnError)
	flags.SetOutput(output)
	path := flags.String("config", "config/app.development.yaml", "YAML configuration path")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("parse arguments: %w", err)
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	cfg, err := config.LoadConfig(*path)
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	app, err := bootstrap.NewApp(ctx, cfg, output)
	if err != nil {
		return fmt.Errorf("start application: %w", err)
	}
	defer func() { runErr = errors.Join(runErr, bootstrap.Close(app)) }()
	return bootstrap.Serve(ctx, app)
}
