// Package wslinstance implements the GRPC WSLInstance service.
package wslinstance

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	agentapi "github.com/canonical/ubuntu-pro-for-wsl/agentapi/go"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/database"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/distro"
	log "github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/grpc/logstreamer"
	"github.com/ubuntu/decorate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// LandscapeController is the  controller for the Landscape client proservice.
type LandscapeController interface {
	SendUpdatedInfo(context.Context) error
}

// Service is the WSL Instance GRPC service implementation.
type Service struct {
	agentapi.UnimplementedWSLInstanceServer

	db        *database.DistroDB
	landscape LandscapeController
}

// New returns a new service handling WSL Instance API.
func New(ctx context.Context, db *database.DistroDB, landscape LandscapeController) (s Service, err error) {
	log.Debug(ctx, "Building new GRPC WSLInstance server")

	return Service{db: db, landscape: landscape}, nil
}

// Connected establishes a connection with a WSL instance and keeps its properties
// in the database up-to-date.
func (s *Service) Connected(stream agentapi.WSLInstance_ConnectedServer) (err error) {
	ctx := stream.Context()

	info, err := stream.Recv()
	if err != nil {
		return fmt.Errorf("WSLInstance service: incomplete handshake: did not receive info from WSL distro: %v", err)
	}

	distroName := info.GetWslName()

	props, err := propsFromInfo(info)
	if err != nil {
		return fmt.Errorf("invalid DistroInfo: %v", err)
	}

	log.Debugf(ctx, "received properties: %v", props)

	d, err := s.db.GetDistroAndUpdateProperties(ctx, distroName, props)
	if err != nil {
		return err
	}

	// Load deferred tasks
	d.EnqueueDeferredTasks()

	// Update landscape when connecting and disconnecting
	s.landscapeSendUpdatedInfo(ctx, distroName)
	defer s.landscapeSendUpdatedInfo(ctx, distroName)

	conn, err := newWslServiceConn(ctx, d.Name(), stream)
	if err != nil {
		return fmt.Errorf("could not connect to Linux-side WSL service: %v", err)
	}

	if err := d.SetConnection(conn); err != nil {
		return err
	}

	//nolint:errcheck // We don't care about this error because we're cleaning up
	defer d.SetConnection(nil)

	log.Debug(ctx, "connection to Linux-side WSL service established")

	// Blocking connection for the lifetime of the WSL service.
	for {
		info, err := stream.Recv()
		if err != nil {
			return fmt.Errorf("could not receive info: %v", err)
		}

		props, err = propsFromInfo(info)
		if err != nil {
			return fmt.Errorf("invalid DistroInfo: %v", err)
		}
		log.Infof(ctx, "Updated properties to %+v", props)

		if d.SetProperties(props) {
			if err := s.db.Dump(); err != nil {
				log.Warningf(ctx, "updating properties: %v", err)
			}
		}

		s.landscapeSendUpdatedInfo(ctx, distroName)
	}
}

type portSender interface {
	Send(*agentapi.Port) error
}

const maxConnectionAttempts = 5

func newWslServiceConn(ctx context.Context, distroName string, send portSender) (conn *grpc.ClientConn, err error) {
	log.Debugf(ctx, "WSLInstance service (%s): reserving a port", distroName)
	for i := 0; i < maxConnectionAttempts && conn == nil; i++ {
		if err != nil {
			log.Warningf(ctx, "WSLInstance service (%s): retrying to reserve a port: %v", distroName, err)
		}
		conn, err = func() (conn *grpc.ClientConn, err error) {
			// Port reservation.
			lis, err := net.Listen("tcp4", "localhost:")
			if err != nil {
				return nil, err
			}

			p, err := getPort(lis)
			if err != nil {
				return nil, err
			}
			log.Debugf(ctx, "WSLInstance service (%s): reserved port %d", distroName, p)

			if err := lis.Close(); err != nil {
				return nil, err
			}

			// Send it to WSL service.
			if err := send.Send(&agentapi.Port{Port: uint32(p)}); err != nil {
				return nil, fmt.Errorf("could not send reserved port: %v", err)
			}

			// Connection.
			addr := fmt.Sprintf("localhost:%d", p)
			log.Debugf(ctx, "WSLInstance service (%s): connecting to Linux-side WSL service via %s", distroName, addr)

			ctxTimeout, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()

			conn, err = grpc.DialContext(ctxTimeout, addr,
				grpc.WithTransportCredentials(insecure.NewCredentials()),
				grpc.WithBlock())
			if err != nil {
				return nil, fmt.Errorf("could not dial WSL service: %v", err)
			}

			// This will signal the task worker that we are ready to process tasks.
			return conn, nil
		}()
	}

	return conn, err
}

func getPort(lis net.Listener) (int, error) {
	_, port, err := net.SplitHostPort(lis.Addr().String())
	if err != nil {
		return 0, fmt.Errorf("could not parse port in address %q: %v", lis.Addr().String(), err)
	}

	return net.LookupPort("tcp4", port)
}

func propsFromInfo(info *agentapi.DistroInfo) (props distro.Properties, err error) {
	defer decorate.OnError(&err, "received invalid distribution info")

	if info.GetWslName() == "" {
		return props, errors.New("no id provided")
	}

	return distro.Properties{
		DistroID:    info.GetId(),
		VersionID:   info.GetVersionId(),
		PrettyName:  info.GetPrettyName(),
		ProAttached: info.GetProAttached(),
		Hostname:    info.GetHostname(),
	}, nil
}

// landscapeSendUpdatedInfo is syntactic sugar to update landscape and
// log in the case error.
func (s *Service) landscapeSendUpdatedInfo(ctx context.Context, distroName string) {
	go func() {
		log.Debugf(ctx, "WSLInstance service (%s): sending updated info to Landscape", distroName)

		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		if err := s.landscape.SendUpdatedInfo(ctx); err != nil {
			log.Warningf(ctx, err.Error())
		}
	}()
}
