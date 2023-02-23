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

	"github.com/canonical/ubuntu-pro-for-windows/agentapi"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distro"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distroDB"
	log "github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/task"
	"github.com/ubuntu/decorate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Service is the WSL Instance GRPC service implementation.
type Service struct {
	agentapi.UnimplementedWSLInstanceServer

	db *distroDB.DistroDB
}

// New returns a new service handling WSL Instance API.
func New(ctx context.Context, db *distroDB.DistroDB) (s Service, err error) {
	log.Debug(ctx, "Building new GRPC WSL Instance service")

	return Service{db: db}, nil
}

// Connected establishes a connection with a WSL instance and keeps its properties
// in the database up-to-date.
func (s *Service) Connected(stream agentapi.WSLInstance_ConnectedServer) error {
	log.Debug(context.TODO(), "New connection detected")

	info, err := stream.Recv()
	if err != nil {
		return fmt.Errorf("new connection: did not receive info from WSL distro: %v", err)
	}

	props, err := propsFromInfo(info)
	if err != nil {
		return fmt.Errorf("connection from %q: invalid DistroInfo: %v", info.WslName, err)
	}

	log.Debugf(context.TODO(), "Connection from %q: received properties: %v", info.WslName, props)

	d, err := s.db.GetDistroAndUpdateProperties(context.TODO(), info.WslName, props)
	if err != nil {
		return fmt.Errorf("connection from %q: %v", info.WslName, err)
	}

	conn, err := newWslServiceConn(context.TODO(), d.Name, stream)
	if err != nil {
		return fmt.Errorf("connection from %q: could not connect to Linux-side service: %v", d.Name, err)
	}

	d.SetConnection(conn)
	defer d.SetConnection(nil)

	log.Debugf(context.TODO(), "Connection to Linux-side service established")

	// TODO: This is for testing, remove it when we're done
	_ = d.SubmitTask(&task.Ping{})

	// Blocking connection for the lifetime of the WSL service.
	for {
		info, err := stream.Recv()
		if err != nil {
			return fmt.Errorf("connection from %q: Failed to receive info: %v", d.Name, err)
		}

		props, err = propsFromInfo(info)
		if err != nil {
			return fmt.Errorf("connection from %q: invalid DistroInfo: %v", d.Name, err)
		}
		log.Infof(context.TODO(), "Connection from %q: Updated properties to %+v", info.WslName, props)

		if d.Properties != props {
			d.Properties = props
			if err := s.db.Dump(); err != nil {
				log.Warningf(context.TODO(), "Connection from %q: could not dump database to disk: %v", info.WslName, err)
			}
		}
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
			lis, err := net.Listen("tcp4", "")
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

			ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
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

	if info.WslName == "" {
		return props, errors.New("no id provided")
	}

	return distro.Properties{
		DistroID:    info.Id,
		VersionID:   info.VersionId,
		PrettyName:  info.PrettyName,
		ProAttached: info.ProAttached,
	}, nil
}
