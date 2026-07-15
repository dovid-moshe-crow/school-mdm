package httpserver

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// Server wraps http.Server with useful defaults.
type Server struct {
	HTTP *http.Server
	Log  *slog.Logger
}

// New creates a server listening on addr with handler.
func New(addr string, handler http.Handler, log *slog.Logger) *Server {
	if log == nil {
		log = slog.Default()
	}
	return &Server{
		HTTP: &http.Server{
			Addr:              addr,
			Handler:           handler,
			ReadHeaderTimeout: 5 * time.Second,
		},
		Log: log,
	}
}

// ListenAndServe starts the server until ctx is cancelled.
func (s *Server) ListenAndServe(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.HTTP.Addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	s.Log.Info("listening", "addr", ln.Addr().String())

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.HTTP.Serve(ln)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.HTTP.Shutdown(shutdownCtx)
		err := <-errCh
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}
