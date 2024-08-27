// Package streams abstracts the bi-directional gRPC stream and provides a faux server that mimics a unary call server.
package streams

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sync"

	agentapi "github.com/canonical/ubuntu-pro-for-wsl/agentapi/go"
	log "github.com/canonical/ubuntu-pro-for-wsl/common/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-wsl/wsl-pro-service/internal/system"
	"google.golang.org/grpc"
)

// CommandService is the interface that the real service must implement to handle the commands received from the control stream.
type CommandService interface {
	ApplyProToken(ctx context.Context, msg *agentapi.ProAttachCmd) error
	ApplyLandscapeConfig(ctx context.Context, msg *agentapi.LandscapeConfigCmd) error
}

// Server is a struct that mimics a unary call server. It is backed by a bi-directional gRPC stream.
//
// It is used to make unary calls from the real gRPC server (Windows Agent) to the real client (this faux server).
type Server struct {
	conn   *grpc.ClientConn
	system *system.System

	done chan struct{}

	ctx    context.Context
	cancel context.CancelFunc

	gracefulCtx    context.Context
	gracefulCancel context.CancelFunc
}

// SystemError is an error caused by a misconfiguration of the system, rather than
// originated from Ubuntu Pro for WSL.
type SystemError struct {
	error
}

// NewSystemError creates a new system error wrapping fmt.Errorf.
func NewSystemError(msg string, args ...any) SystemError {
	return SystemError{fmt.Errorf(msg, args...)}
}

func (err SystemError) Error() string {
	return err.error.Error()
}

// Is makes it so all SystemError match SystemError{}.
func (err SystemError) Is(e error) bool {
	_, ok := e.(SystemError)
	return ok
}

// NewServer creates a new Server.
func NewServer(ctx context.Context, sys *system.System, conn *grpc.ClientConn) *Server {
	fCtx, cancel := context.WithCancel(ctx)
	gCtx, gCancel := context.WithCancel(ctx)

	s := &Server{
		conn:   conn,
		system: sys,
		done:   make(chan struct{}),

		// the stream context will be a child of forcequit context and will thus be cancelled with it.
		ctx:    fCtx,
		cancel: cancel,

		gracefulCtx:    gCtx,
		gracefulCancel: gCancel,
	}

	return s
}

// Stop stops the server and the underlying connection immediately.
// It blocks until the server finishes its teardown.
func (s *Server) Stop() {
	s.cancel()
	<-s.done
}

// GracefulStop stops the server as soon as all active unary calls finish.
// It blocks until the server finishes its teardown.
func (s *Server) GracefulStop() {
	s.gracefulCancel()
	<-s.done
}

// Serve starts receiving commands from the control stream and forwards them to the provided service.
// It blocks until stops serving.
func (s *Server) Serve(service CommandService) error {
	defer s.cancel()
	defer close(s.done)

	client, err := connect(s.ctx, s.conn)
	if err != nil {
		return fmt.Errorf("could not start serving: could not connect: %v", err)
	}

	ch := make(chan error)
	var wg sync.WaitGroup

	for _, h := range []handler{
		newHandler(client.ProAttachStream(), service.ApplyProToken),
		newHandler(client.LandscapeConfigStream(), service.ApplyLandscapeConfig),
	} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch <- h.run(s, client)

			// Gracefully stop other handlers once any of them exits.
			s.gracefulCancel()
		}()
	}

	// Notify Agent that we are ready
	info, err := s.system.Info(s.ctx)
	if err != nil {
		return NewSystemError("could not serve: %v", err)
	}

	if err := client.SendInfo(info); err != nil {
		return fmt.Errorf("could not serve: could not send first Connnected message: %v", err)
	}

	if err := client.ProAttachStream().SendWslName(info.GetWslName()); err != nil {
		return fmt.Errorf("could not serve: could not send first ProAttachCmd message: %v", err)
	}

	if err := client.LandscapeConfigStream().SendWslName(info.GetWslName()); err != nil {
		return fmt.Errorf("could not serve: could not send first LandscapeConfigCmd message: %v", err)
	}

	log.Debug(s.ctx, "Server: sent preface messages to all streams")

	go func() {
		wg.Wait()
		close(ch)
	}()

	err = nil
	for msg := range ch {
		err = errors.Join(err, msg)
	}
	if err != nil {
		return fmt.Errorf("serve error: %w", err)
	}

	return nil
}

// handler interface for type erasure: it allows for having all handlerImpl in the same slice.
type handler interface {
	run(s *Server, client *multiClient) error
}

// newHandler takes the ingredients for a handler and hides their type under the type-erased handler.
// This is essentially a handler factory.
func newHandler[Command any](stream stream[Command], callback func(context.Context, *Command) error) handler {
	return &handlingLoop[Command]{
		stream:   stream,
		callback: callback,
	}
}

// handlingLoop implements the logic of the request handling loop.
type handlingLoop[Command any] struct {
	stream   stream[Command]
	callback func(context.Context, *Command) error
}

func (h *handlingLoop[Command]) run(s *Server, client *multiClient) error {
	// Use this context to log onto the stream, and to cancel with server.Stop
	ctx, cancel := cancelWith(h.stream.Context(), s.ctx)
	defer cancel()

	// Use this context to log onto the stream, but cancel with server.GracefulStop
	gCtx, cancel := cancelWith(ctx, s.gracefulCtx)
	defer cancel()

	for {
		// Graceful stop
		select {
		case <-gCtx.Done():
			return nil
		default:
		}

		log.Debugf(ctx, "Started serving %s requests", reflect.TypeFor[Command]())

		// Handle a single command
		msg, ok, err := receiveWithContext(gCtx, h.stream.Recv)
		if err != nil {
			return fmt.Errorf("could not receive ProAttachCmd: %w", err)
		} else if !ok {
			// Non-erroneous exit. Probably a graceful stop.
			return nil
		}

		result := h.callback(ctx, msg)

		if err := h.stream.SendResult(result); err != nil {
			return fmt.Errorf("could not send ProAttachCmd result: %w", err)
		}

		// Send back updated info after command completion
		info, err := s.system.Info(ctx)
		if err != nil {
			log.Warningf(ctx, "Streamserver: could not gather info after command completion: %v", err)
		}

		if err = client.SendInfo(info); err != nil {
			log.Warningf(ctx, "Streamserver: could not stream back info after command completion")
		}
	}
}

// cancelWith creates a child context that is cancelled when with is done.
func cancelWith(ctx, with context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(ctx)
	context.AfterFunc(with, cancel)
	return ctx, cancel
}

// Receive with context calls the recv receiver asyncronously.
// Returns (message, message error) if recv returned.
// Returns (nil, context error) if the context was cancelled.
func receiveWithContext[MessageT any](ctx context.Context, recv func() (*MessageT, error)) (*MessageT, bool, error) {
	select {
	case <-ctx.Done():
		return nil, false, ctx.Err()
	default:
	}

	type retval struct {
		t   *MessageT
		err error
	}
	ch := make(chan retval)

	go func() {
		defer close(ch)
		t, err := recv()
		ch <- retval{t, err}
	}()

	select {
	case <-ctx.Done():
		return nil, false, nil
	case msg := <-ch:
		if errors.Is(msg.err, io.EOF) {
			return nil, false, nil
		}
		return msg.t, true, msg.err
	}
}
