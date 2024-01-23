// Package wslinstance implements the GRPC WSLInstance service.
package wslinstance

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	agentapi "github.com/canonical/ubuntu-pro-for-windows/agentapi/go"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/database"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/distro"
	log "github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/grpc/logstreamer"
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
	log.Debug(ctx, "Building new GRPC WSL Instance service")

	return Service{db: db, landscape: landscape}, nil
}

// Connected establishes a connection with a WSL instance and keeps its properties
// in the database up-to-date.
func (s *Service) Connected(stream agentapi.WSLInstance_ConnectedServer) error {
	ctx := context.TODO()

	// TODO: fix it.
	defer log.Debug(ctx, "Connection dropped")
	log.Debug(ctx, "New connection detected")

	info, err := stream.Recv()
	if err != nil {
		return fmt.Errorf("new connection: did not receive info from WSL distro: %v", err)
	}

	props, err := propsFromInfo(info)
	if err != nil {
		return fmt.Errorf("connection from %q: invalid DistroInfo: %v", info.GetWslName(), err)
	}

	log.Debugf(ctx, "Connection from %q: received properties: %v", info.GetWslName(), props)

	d, err := s.db.GetDistroAndUpdateProperties(ctx, info.GetWslName(), props)
	if err != nil {
		return fmt.Errorf("connection from %q: %v", info.GetWslName(), err)
	}

	// Load deferred tasks
	d.EnqueueDeferredTasks()

	// Update landscape when connecting and disconnecting
	s.landscapeSendUpdatedInfo(ctx)
	defer s.landscapeSendUpdatedInfo(ctx)

	conn, err := newWslServiceConn(ctx, d.Name(), stream)
	if err != nil {
		return fmt.Errorf("connection from %q: could not connect to Linux-side service: %v", d.Name(), err)
	}

	if err := d.SetConnection(conn); err != nil {
		return fmt.Errorf("connection from %q: %v", info.GetWslName(), err)
	}

	//nolint:errcheck // We don't care about this error because we're cleaning up
	defer d.SetConnection(nil)

	log.Debugf(ctx, "Connection to Linux-side service established")

	// Blocking connection for the lifetime of the WSL service.
	for {
		info, err := stream.Recv()
		if err != nil {
			return fmt.Errorf("connection from %q: Failed to receive info: %v", d.Name(), err)
		}

		props, err = propsFromInfo(info)
		if err != nil {
			return fmt.Errorf("connection from %q: invalid DistroInfo: %v", d.Name(), err)
		}
		log.Infof(ctx, "Connection from %q: Updated properties to %+v", d.Name(), props)

		if d.SetProperties(props) {
			if err := s.db.Dump(); err != nil {
				log.Warningf(ctx, "Connection from %q: could not dump database to disk: %v", d.Name(), err)
			}
		}

		s.landscapeSendUpdatedInfo(ctx)
	}
}

type portSender interface {
	Send(*agentapi.Port) error
}

const maxConnectionAttempts = 5

func newWslServiceConn(ctx context.Context, distroName string, send portSender) (conn *grpc.ClientConn, err error) {
	log.Debugf(ctx, "Connection from %q: Reserving a port", distroName)
	for i := 0; i < maxConnectionAttempts && conn == nil; i++ {
		if err != nil {
			log.Warningf(ctx, "Connection from %q: Retrying to reserve a port: %v", distroName, err)
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
			log.Debugf(ctx, "Connection from %q: Reserved port %d", distroName, p)

			if err := lis.Close(); err != nil {
				return nil, err
			}

			// Send it to WSL service.
			if err := send.Send(&agentapi.Port{Port: p}); err != nil {
				return nil, err
			}

			// Connection.
			addr := fmt.Sprintf("localhost:%d", p)
			log.Debugf(ctx, "Connection from %q: connecting to Linux-side service via %s", distroName, addr)

			ctxTimeout, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()

			conn, err = grpc.DialContext(ctxTimeout, addr,
				grpc.WithTransportCredentials(insecure.NewCredentials()),
				grpc.WithBlock())
			if err != nil {
				return nil, fmt.Errorf("could not contact the grpc server for %q: %v", distroName, err)
			}

			// This will signal the task worker that we are ready to process tasks.
			return conn, nil
		}()
	}

	return conn, err
}

func getPort(lis net.Listener) (uint32, error) {
	tmp := strings.Split(lis.Addr().String(), ":")
	port, err := strconv.ParseUint(tmp[len(tmp)-1], 10, 16)
	if err != nil {
		return 0, fmt.Errorf("could not parse port in address %q: %v", lis.Addr().String(), err)
	}
	return uint32(port), nil
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
func (s *Service) landscapeSendUpdatedInfo(ctx context.Context) {
	go func() {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		if err := s.landscape.SendUpdatedInfo(ctx); err != nil {
			log.Warningf(ctx, err.Error())
		}
	}()
}
