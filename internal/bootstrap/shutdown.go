package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"

	"sea-codex-server/internal/common"
)

// Serve owns the HTTP lifecycle. The caller closes database dependencies afterwards.
func Serve(ctx context.Context, app *common.App) error {
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
