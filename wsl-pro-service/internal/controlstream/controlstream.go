// Package controlstream encapsulates details of the connection to the control stream served by the Windows Agent.
package controlstream

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	agentapi "github.com/canonical/ubuntu-pro-for-windows/agentapi/go"
	log "github.com/canonical/ubuntu-pro-for-windows/wsl-pro-service/internal/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-windows/wsl-pro-service/internal/system"
	"github.com/ubuntu/decorate"
	"google.golang.org/grpc/connectivity"
)

// ControlStream manages the connection to the control stream served by the Windows Agent.
type ControlStream struct {
	system   system.System
	addrPath string
	session  session
	port     uint32
}

// SystemError is an error caused by a misconfiguration of the system, rather than
// originated from Ubuntu Pro for Windows.
type SystemError struct {
	error
}

func systemErrorf(msg string, args ...any) SystemError {
	return SystemError{fmt.Errorf(msg, args...)}
}

func (err SystemError) Error() string {
	return err.error.Error()
}

// New creates an iddle control stream object.
func New(agentPortFilePath string, s system.System) ControlStream {
	return ControlStream{
		addrPath: agentPortFilePath,
		system:   s,
	}
}

// Connect connects to the control stream. Call Disconnect to release resources.
func (cs *ControlStream) Connect(ctx context.Context) (err error) {
	defer decorate.OnError(&err, "could not connect to windows agent via the control stream")

	ctrlAddr, err := cs.address(ctx)
	if err != nil {
		return fmt.Errorf("could not get address: %w", err)
	}

	session, err := newSession(ctx, ctrlAddr)
	if err != nil {
		return err
	}

	log.Debug(ctx, "Connected to Windows agent via the control stream")

	port, err := cs.handshake(ctx, session)
	if err != nil {
		return err
	}

	log.Debug(ctx, "Completed handshake with Windows agent via the control stream")

	cs.session = session
	cs.port = port

	return nil
}

func (cs *ControlStream) handshake(ctx context.Context, session session) (port uint32, err error) {
	defer decorate.OnError(&err, "could not complete handshake")

	sysinfo, err := cs.system.Info(ctx)
	if err != nil {
		return 0, systemErrorf("could not obtain system info: %v", err)
	}

	if err := session.send(sysinfo); err != nil {
		return 0, fmt.Errorf("could not send system info: %v", err)
	}

	message, err := session.recv()
	if err != nil {
		return 0, fmt.Errorf("could not receive: %v", err)
	}

	p := message.GetPort()
	if p == 0 {
		return 0, errors.New("received invalid message: port cannot be zero")
	}

	return p, nil
}

// Disconnect dumps the existing connection (if any). The connection can be re-established by calling Connect.
func (cs *ControlStream) Disconnect() {
	cs.session.close()
	cs.port = 0
}

// address fetches the address of the control stream from the Windows filesystem.
func (cs ControlStream) address(ctx context.Context) (string, error) {
	windowsLocalhost, err := cs.system.WindowsHostAddress(ctx)
	if err != nil {
		return "", SystemError{err}
	}

	/*
		We parse the port from the file written by the windows agent.
	*/
	addr, err := os.ReadFile(cs.addrPath)
	if err != nil {
		return "", fmt.Errorf("could not read agent port file %q: %v", cs.addrPath, err)
	}

	fields := strings.Split(string(addr), ":")
	if len(fields) == 0 {
		// Avoid a panic. As far as I know, there is no way of triggering this,
		// but we may as well protect against it.
		return "", fmt.Errorf("could not extract port out of address %q", addr)
	}
	port := fields[len(fields)-1]

	return fmt.Sprintf("%s:%s", windowsLocalhost, port), nil
}

// ReservedPort returns the port assigned to this distro.
func (cs ControlStream) ReservedPort() uint32 {
	return cs.port
}

// Send sends info about the system to the Windows Agent.
func (cs ControlStream) Send(info *agentapi.DistroInfo) error {
	return cs.session.send(info)
}

// Done returns a channel that blocks for as long as the connection to the stream lasts.
// Cancel the context to release resources.
func (cs ControlStream) Done(ctx context.Context) <-chan struct{} {
	ch := make(chan struct{})

	conn := cs.session.conn
	if conn == nil {
		close(ch)
		return ch
	}

	go func() {
		defer close(ch)
		conn.WaitForStateChange(ctx, connectivity.Ready)
	}()
	return ch
}
