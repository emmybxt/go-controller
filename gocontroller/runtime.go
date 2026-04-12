package gocontroller

import (
	"context"
	"errors"
	"net/http"
	"time"
)

type ServerOptions struct {
	Addr            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}

func (a *App) NewHTTPServer(opts ServerOptions) *http.Server {
	addr := opts.Addr
	if addr == "" {
		addr = ":8080"
	}
	return &http.Server{
		Addr:         addr,
		Handler:      a.Handler(),
		ReadTimeout:  opts.ReadTimeout,
		WriteTimeout: opts.WriteTimeout,
		IdleTimeout:  opts.IdleTimeout,
	}
}

// Run starts the HTTP server and gracefully shuts down on context cancellation.
func (a *App) Run(ctx context.Context, opts ServerOptions) error {
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

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownTimeout := opts.ShutdownTimeout
		if shutdownTimeout <= 0 {
			shutdownTimeout = 10 * time.Second
		}
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return nil
	}
}
