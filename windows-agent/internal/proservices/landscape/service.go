// Package landscape implements a client to the Landscape Host Agent API service.
package landscape

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	landscapeapi "github.com/canonical/landscape-hostagent-api"
	log "github.com/canonical/ubuntu-pro-for-wsl/common/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/config"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/database"
	"github.com/ubuntu/decorate"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/status"
)

// Service orchestrates the Landscape hostagent connection. It lasts for the entire lifetime of the program.
// It creates the executor and ensures there is always an active connection, creating a new one if necessary.
type Service struct {
	ctx     context.Context
	cancel  context.CancelFunc
	running chan struct{}

	disabled atomic.Bool

	db   *database.DistroDB
	conf Config

	// Cached hostName
	hostName string
	// Where to store persistent artifacts, such as an imported WSL VHDX.
	homedir string
	// Where to download temporary artifacts
	downloaddir string

	// Connection
	conn   *connection
	connMu sync.RWMutex

	// connRetrier is used in order to ask the keepConnected
	// function to try again now (instead of waiting for the retrial
	// time). Do not use directly. Instead use reconnect().
	connRetrier *retryConnection

	cloudinit CloudInit

	notifyConnectionState ConnStateListener
}

// ConnStateListener is a non-blocking callback that will be invoked for any interesting connectivity events,
// such as terminal errors due invalid configuration or `nil` for successful connections.
type ConnStateListener func(context.Context, error)

// Config is a configuration provider for ProToken and the Landscape URL.
type Config interface {
	LandscapeClientConfig() (string, config.Source, error)

	Subscription() (string, config.Source, error)

	LandscapeAgentUID() (string, error)
	SetLandscapeAgentUID(context.Context, string) error
}

// CloudInit is a cloud-init user data writer.
type CloudInit interface {
	WriteDistroData(distroName string, cloudInit string, instanceID string) error
	RemoveDistroData(distroName string) error
}

type options struct {
	hostname    string
	homedir     string
	downloaddir string
}

// Option is an optional argument for NewClient.
type Option = func(*options)

// New creates a new Landscape service object.
func New(ctx context.Context, conf Config, db *database.DistroDB, cloudInit CloudInit, notifyConnectionState ConnStateListener, args ...Option) (s *Service, err error) {
	defer decorate.OnError(&err, "could not initizalize Landscape service")
	var opts options

	for _, f := range args {
		f(&opts)
	}

	if opts.hostname == "" {
		hostname, err := os.Hostname()
		if err != nil {
			return nil, fmt.Errorf("could not get host name: %v", err)
		}
		opts.hostname = hostname
	}

	if opts.homedir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("could not locate the user home dir: %v", err)
		}

		opts.homedir = homeDir
	}

	if opts.downloaddir == "" {
		opts.downloaddir = os.TempDir()
	}

	ctx, cancel := context.WithCancel(ctx)

	if notifyConnectionState == nil {
		notifyConnectionState = func(context.Context, error) {}
	}

	s = &Service{
		ctx:                   ctx,
		cancel:                cancel,
		conf:                  conf,
		db:                    db,
		hostName:              opts.hostname,
		homedir:               opts.homedir,
		downloaddir:           opts.downloaddir,
		connRetrier:           newRetryConnection(),
		cloudinit:             cloudInit,
		notifyConnectionState: notifyConnectionState,
	}

	return s, nil
}

// Connect starts the connection and starts talking to the server.
// Call Stop to deallocate resources.
func (s *Service) Connect() (err error) {
	if s.connected() {
		return errors.New("could not connect to Landscape server: already connected")
	}

	if err := s.keepConnected(); errors.Is(err, noConfigError{}) {
		return nil
	} else if err != nil {
		return fmt.Errorf("could not connect to Landscape server: %v", err)
	}

	return nil
}

// keepConnected supervises the connection. It attempts connecting before returning.
// The connection will be re-created if:
// - the active one drops.
// - a reconnection is requested via connRetrier.
func (s *Service) keepConnected() error {
	const growthFactor = 2
	const minWait = time.Second
	const maxWait = 10 * time.Minute
	wait := 0 * time.Second // No wait in the first iteration

	s.running = make(chan struct{})
	started := make(chan error)

	go func() {
		defer close(s.running)

		defer s.disconnect()
		first := true

		for {
			err := func() error {
				var waitCh <-chan time.Time

				if !s.disabled.Load() {
					cooldown := time.NewTimer(wait)
					defer cooldown.Stop()
					waitCh = cooldown.C
					if wait > minWait {
						log.Infof(s.ctx, "Landscape will attempt to connect in %s", wait)
					}
				}

				select {
				case <-s.ctx.Done():
					return s.ctx.Err()
				case <-s.connRetrier.Await():
				case <-waitCh:
					// We use the cooldown to see if the connection is long-lived.
					// Short-lived connections will be considered a failure.
					// This avoids spamming the server with short-lived connections.
					cooldown := time.NewTimer(wait)
					defer cooldown.Stop()
					waitCh = cooldown.C
				}

				// Retrial petitions are all satisfied.
				s.connRetrier.Reset()

				connectionDone, err := s.connectOnce(s.ctx)

				if first {
					started <- err
					close(started)
					wait = minWait
					first = false
				}

				if err != nil {
					return err
				}

				log.Info(s.ctx, "Landscape: connected")
				s.disabled.Store(false)

				select {
				case <-s.ctx.Done():
					return s.ctx.Err()
				case <-s.connRetrier.Await():
					log.Info(s.ctx, "Landscape: reconnection requested")
					// this also causes the loop to break because the next iteration will fall into one of the other cases.
					s.disconnect()
				case err := <-connectionDone:
					select {
					case <-waitCh:
						// Connection was dropped so fast we'll consider it a failure.
						return errors.New("connection dropped unexpectedly")
					default:
					}

					return err
				}

				return nil
			}()

			select {
			case <-s.ctx.Done():
				log.Info(s.ctx, "Landscape: stopped by context")
				return
			default:
			}

			if checkErr := mustDisableService(err); checkErr != nil {
				if !s.disabled.Load() {
					// "Landscape: service disabled" not already logged.
					log.Warningf(s.ctx, "Landscape: %v", checkErr)
					s.disabled.Store(true)
				}
				continue
			}

			if err != nil {
				log.Warningf(s.ctx, "Landscape: %v", err)
				wait = min(growthFactor*wait, maxWait)
				continue
			}

			// Connection was long-lived. We don't need to wait before reconnecting.
			wait = minWait
		}
	}()

	select {
	case <-s.ctx.Done():
		return s.ctx.Err()
	case err := <-started:
		return err
	}
}

// mustDisableService returns an error that wraps err if it requires disabling the service, otherwise it returns nil.
func mustDisableService(err error) error {
	// Service must remain disabled if we don't have a Landscape config.
	if target := (noConfigError{}); errors.As(err, &target) {
		return fmt.Errorf("service disabled: %w", target)
	}
	// Or if the server rejects our request.
	if status.Code(err) == codes.PermissionDenied || status.Code(err) == codes.InvalidArgument {
		return fmt.Errorf("service disabled: %w", err)
	}
	// Or if the DNS server doesn't find the host.
	if status.Code(err) == codes.Unavailable {
		// I wish there was an idiomatic way in gRPC to check if calling a DNS server succeeded but it server couldn't find the host.
		if strings.Contains(err.Error(), "produced zero addresses") || strings.Contains(err.Error(), "no such host") {
			return fmt.Errorf("service disabled: %w", err)
		}
	}

	return nil
}

func (s *Service) connectOnce(ctx context.Context) (<-chan error, error) {
	s.connMu.Lock()
	defer s.connMu.Unlock()

	if s.conn != nil {
		s.conn.disconnect()
		s.conn = nil
	}

	previousUID, err := s.conf.LandscapeAgentUID()
	if err != nil {
		return nil, err
	}

	conn, err := newConnection(ctx, s)
	if err != nil {
		// No config error is not interesting for listeners, it's just an implementation detail.
		if !errors.Is(err, &noConfigError{}) {
			s.notifyConnectionState(ctx, err)
		}
		return nil, err
	}

	// Cancelled errors are expected when the hostagent UID changes.
	// On reconnection a new handshake() happens but the UID remains the same.
	// To detect meaningful errors that need to be broadcasted, we check if the UID has changed
	// to a non-empty value.
	newUID, err := s.conf.LandscapeAgentUID()
	if err != nil {
		return nil, err
	}
	unchangedUID := newUID == previousUID && newUID != ""

	// If we reached this point, the handshake() with the Landscape server completed
	// successfully, we have a hostagent UID and we are listening for commands. But failures can happen
	// at any point in time and the connection be terminated. The following monitoring will
	// tell us if everything is fine or if we have a problem.
	// This goroutine assumes the Cancelled status code is only used for the `Reconnect with uid set` message.
	connectionDone := make(chan error, 1)
	go func() {
		defer close(connectionDone)

		connStatus := connectivity.Connecting // Don't do GetState() just in case we already failed.
		for {
			conn.grpcConn.WaitForStateChange(ctx, connStatus)
			connStatus = conn.grpcConn.GetState()

			// Ideally if we passed through the handshake() successfully, the gRPC stream should be guaranteed to be good.
			// But as of now the server performs config validation at a later point, so we may have received a hostagent UID
			// and be surprised with a Recv failure right after processing the initial commands.
			// To avoid this, let's give some time waiting for errors to come from the commandErrs channel.
			// TODO: simplify this once LDENG-3037 lands in the server (hopefully without the time based programming).
			if unchangedUID && (connStatus == connectivity.Idle || connStatus == connectivity.Ready) {
				select {
				case <-time.After(500 * time.Millisecond):
					// Stream is up and running, let's notify listeners of this success.
					s.notifyConnectionState(ctx, nil)
					continue
				case err := <-conn.commandErrs:
					// We got an error, this connection is done.
					if status.Code(err) != codes.Canceled {
						s.notifyConnectionState(ctx, err)
					}
					connectionDone <- err
					return
				}
			}

			// Otherwise we already received a terminal error. Let's bail out and notify listeners
			// in case it's not a `code=Cancelled desc=Reconnect with uid set`.
			if connStatus == connectivity.Shutdown {
				// We got an error, this connection is done.
				err := <-conn.commandErrs
				connectionDone <- err
				// As before, listeners won't care about the Cancelled status nor errors caused by UID changes.
				if unchangedUID || status.Code(err) != codes.Canceled {
					s.notifyConnectionState(ctx, err)
				}
				return
			}
		}
	}()

	s.conn = conn
	return connectionDone, nil
}

// Stop terminates the connection and deallocates resources.
func (s *Service) Stop(ctx context.Context) {
	log.Infof(ctx, "Landscape: stopping")

	s.cancel()
	s.connRetrier.Stop()

	select {
	case <-s.running:
	case <-ctx.Done():
	}
}

// Controller creates a controler for this service.
func (s *Service) Controller() Controller {
	return Controller{
		serviceConn: s,
		serviceData: s,
	}
}

// NotifyUbuntuProUpdate is called when the Ubuntu Pro token changes. It will trigger a reconnection if needed.
func (s *Service) NotifyUbuntuProUpdate(ctx context.Context, token string) {
	s.reconnectIfNewSettings(ctx)
}

// NotifyConfigUpdate is called when the configuration changes. It will trigger a reconnection if needed.
func (s *Service) NotifyConfigUpdate(ctx context.Context, landscapeConf, agentUID string) {
	// We only enable Landscape if there is a UID. Otherwise we disable it (by sending an empty config).
	if agentUID == "" {
		landscapeConf = ""
	}

	if landscapeConf != "" {
		var err error
		landscapeConf, err = filterClientSection(landscapeConf)
		if err != nil {
			log.Errorf(ctx, "Landscape: could not notify config changes: %v", err)
			return
		}
	}

	distributeConfig(ctx, s.db, landscapeConf)
	s.reconnectIfNewSettings(ctx)
}

func (s *Service) reconnectIfNewSettings(ctx context.Context) {
	oldSettings := func() landscapeHostConf {
		s.connMu.RLock()
		defer s.connMu.RUnlock()

		if s.conn != nil {
			return s.conn.settings
		}
		return landscapeHostConf{}
	}()

	hostagentConf, err := newLandscapeHostConf(s.conf)
	if err != nil && !errors.Is(err, noConfigError{}) {
		log.Warningf(ctx, "Landscape: config monitor: %v", err)
		return
	}

	// This check is still useful for changes in fields like `[client].log_level`, only meaningful for the WSL instances, not for the agent.
	if hostagentConf == oldSettings {
		// Prevents the UI from being stuck in the rare cases when we have a good connection and receive a config change
		// that only affects the landscape-client inside the WSL instances.
		s.notifyConnectionState(ctx, status.Error(codes.AlreadyExists, "Landscape: already connected with the same settings"))
		return
	}

	s.reconnect()
}

func (s *Service) disconnect() {
	s.connMu.Lock()
	if s.conn != nil {
		s.conn.disconnect()
	}
	s.connMu.Unlock()
}

// The following methods expose some internals for the other components to use.

// reconnect signals the Landscape client to attempt to connect to Landscape.
// It will not block if there is an ative connection. until the reconnect petition
// has been received.
func (s *Service) reconnect() {
	s.connRetrier.Request()
}

func (s *Service) connDone() <-chan struct{} {
	s.connMu.RLock()
	defer s.connMu.RUnlock()

	if s.conn == nil {
		ch := make(chan struct{})
		close(ch)
		return ch
	}

	return s.conn.ctx.Done()
}

func (s *Service) hasStopped() <-chan struct{} {
	return s.ctx.Done()
}

func (s *Service) config() Config {
	return s.conf
}

func (s *Service) database() *database.DistroDB {
	return s.db
}

func (s *Service) hostname() string {
	return s.hostName
}

func (s *Service) homeDir() string {
	return s.homedir
}

func (s *Service) downloadDir() string {
	return s.downloaddir
}

func (s *Service) connected() bool {
	s.connMu.RLock()
	defer s.connMu.RUnlock()

	return s.conn.connected()
}

func (s *Service) sendInfo(info *landscapeapi.HostAgentInfo) error {
	s.connMu.RLock()
	defer s.connMu.RUnlock()

	return s.conn.sendInfo(info)
}

func (s *Service) isDisabled() bool {
	return s.disabled.Load()
}

func (s *Service) cloudInit() CloudInit {
	return s.cloudinit
}
