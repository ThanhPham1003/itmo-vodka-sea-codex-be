package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"sea-codex-server/internal/common"
)

// Serve owns the HTTP lifecycle. The caller closes database dependencies afterwards.
func Serve(ctx context.Context, app *common.App) (serveResult error) {
	if app == nil || app.Config == nil || app.Router == nil {
		return fmt.Errorf("%w: HTTP dependencies are missing", common.ErrInvalidArgument)
	}
	cfg := app.Config.HTTPServer
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", cfg.Address())
	if err != nil {
		return fmt.Errorf("listen for HTTP: %w", err)
	}
	server := &http.Server{
		Handler: app.Router, ReadTimeout: cfg.ReadTimeout, ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		WriteTimeout: cfg.WriteTimeout, IdleTimeout: cfg.IdleTimeout,
	}
	defer server.Close()
	workerCtx, cancelWorkers := context.WithCancel(ctx)
	var workers sync.WaitGroup
	for _, worker := range app.Workers {
		workers.Go(func() {
			if err := worker.Run(workerCtx); err != nil && workerCtx.Err() == nil {
				app.Logger.Error().Msg("background worker stopped")
			}
		})
	}
	defer func() {
		cancelWorkers()
		done := make(chan struct{})
		go func() { workers.Wait(); close(done) }()
		timer := time.NewTimer(cfg.ShutdownTimeout)
		defer timer.Stop()
		select {
		case <-done:
		case <-timer.C:
			serveResult = errors.Join(serveResult, fmt.Errorf("background worker shutdown timed out"))
		}
	}()
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(listener) }()
	app.Logger.Info().Str("address", listener.Addr().String()).Msg("HTTP server started")
	select {
	case err := <-serveErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve HTTP: %w", err)
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown HTTP: %w", err)
		}
		if err := <-serveErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("stop HTTP: %w", err)
		}
		app.Logger.Info().Msg("HTTP server stopped")
		return nil
	}
}
