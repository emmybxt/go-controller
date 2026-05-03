package gocontroller

import (
	"context"
	"errors"
	"net/http"
	"time"
)

type ServerOptions struct {
	Addr              string
	ReadTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
	MaxHeaderBytes    int
}

func (a *App) NewHTTPServer(opts ServerOptions) *http.Server {
	addr := opts.Addr
	if addr == "" {
		addr = ":8080"
	}
	return &http.Server{
		Addr:              addr,
		Handler:           a.Handler(),
		ReadTimeout:       durationOrDefault(opts.ReadTimeout, 15*time.Second),
		ReadHeaderTimeout: durationOrDefault(opts.ReadHeaderTimeout, 5*time.Second),
		WriteTimeout:      durationOrDefault(opts.WriteTimeout, 30*time.Second),
		IdleTimeout:       durationOrDefault(opts.IdleTimeout, 120*time.Second),
		MaxHeaderBytes:    intOrDefault(opts.MaxHeaderBytes, http.DefaultMaxHeaderBytes),
	}
}

// Run starts the HTTP server and gracefully shuts down on context cancellation.
func (a *App) Run(ctx context.Context, opts ServerOptions) error {
	if a.Lifecycle != nil {
		if err := a.Lifecycle.Init(ctx); err != nil {
			return err
		}
	}
	if a.Health != nil {
		a.Health.MarkReady()
	}

	srv := a.NewHTTPServer(opts)
	errCh := make(chan error, 1)

	go func() {
		err := srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	var result error
	select {
	case result = <-errCh:
	case <-ctx.Done():
		shutdownTimeout := opts.ShutdownTimeout
		if shutdownTimeout <= 0 {
			shutdownTimeout = 10 * time.Second
		}
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			result = err
		}
	}

	if a.Lifecycle != nil {
		destroyCtx := context.Background()
		if err := a.Lifecycle.Destroy(destroyCtx); err != nil && result == nil {
			result = err
		}
	}

	return result
}

func durationOrDefault(value, fallback time.Duration) time.Duration {
	if value > 0 {
		return value
	}
	return fallback
}

func intOrDefault(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}
