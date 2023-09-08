// Package restserver provides building blocks to implement a mocked version of out-of-process components Ubuntu Pro For Windows depend on that talk REST,
// such as MS Store API and the Contracts Server backend
// DO NOT USE IN PRODUCTION
package restserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"log/slog"
)

// ServerBase is a configurable mock of the MS Store runtime component that talks REST.
type ServerBase struct {
	server *http.Server
	mu     sync.RWMutex

	done       chan struct{}
	GetAddress func() string
	Mux        *http.ServeMux
}

// Endpoint contains settings for an API endpoint behaviour. Can be modified for testing purposes.
type Endpoint struct {
	// OnSuccess is the response returned in the happy path.
	OnSuccess Response

	// Disabled disables the endpoint.
	Disabled bool

	// Blocked means that a response will not be sent back, instead it'll block until the server is stopped.
	Blocked bool
}

func EndpointOk() Endpoint {
	return Endpoint{OnSuccess: Response{Status: http.StatusOK}}
}

// Response contains settings for an API endpoint response behaviour. Can be modified for testing purposes.
type Response struct {
	Value  string
	Status int
}

// Stop stops the server.
func (s *ServerBase) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.server == nil {
		return errors.New("already stopped")
	}

	err := s.server.Close()
	<-s.done

	s.server = nil

	return err
}

// Serve starts a new HTTP server mocking the MS Store API with
// responses defined according to Server Settings. Use Stop to Stop the server and
// release resources.
func (s *ServerBase) Serve(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.server != nil {
		return "", errors.New("already serving")
	}

	var lc net.ListenConfig
	lis, err := lc.Listen(ctx, "tcp", s.GetAddress())
	if err != nil {
		return "", fmt.Errorf("failed to listen over tcp: %v", err)
	}

	s.server = &http.Server{
		Addr:              lis.Addr().String(),
		Handler:           s.Mux,
		ReadHeaderTimeout: 3 * time.Second,
	}

	s.done = make(chan struct{})

	go func() {
		defer close(s.done)
		if err := s.server.Serve(lis); err != nil && err != http.ErrServerClosed {
			slog.Error("Failed to start the HTTP server", "error", err)
		}
	}()

	return lis.Addr().String(), nil
}

// validateRequest extracts common boilerplate used to validate the request from endpoints.
func (s *ServerBase) ValidateRequest(w http.ResponseWriter, r *http.Request, wantMethod string, endpoint Endpoint) (err error) {
	slog.Info("Received request", "endpoint", r.URL.Path, "method", r.Method)
	defer func() {
		if err != nil {
			slog.Error("bad request", "error", err, "endpoint", r.URL.Path, "method", r.Method)
		}
	}()

	if r.Method != wantMethod {
		w.WriteHeader(http.StatusBadRequest)
		return fmt.Errorf("this endpoint only supports %s", wantMethod)
	}

	if endpoint.Blocked {
		<-s.done
		slog.Debug("Server context was cancelled. Exiting", "endpoint", r.URL.Path)
		return errors.New("server stopped")
	}

	if endpoint.OnSuccess.Status != http.StatusOK {
		w.WriteHeader(endpoint.OnSuccess.Status)
		return fmt.Errorf("mock error: %d", endpoint.OnSuccess.Status)
	}

	return nil
}
