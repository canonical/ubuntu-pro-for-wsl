package landscape

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	landscapeapi "github.com/canonical/landscape-hostagent-api"
	log "github.com/canonical/ubuntu-pro-for-wsl/common/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/database"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/distro"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/tasks"
	"github.com/ubuntu/decorate"
	"github.com/ubuntu/gowsl"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"gopkg.in/ini.v1"
)

// landscapeHostConf is the subset of the landscape configuration relevant to the agent.
type landscapeHostConf struct {
	sslPublicKey    string
	accountName     string
	registrationKey string
	hostagentURL    string
	ubuntuProToken  string
}

type noConfigError struct {
	missing string
}

func (e noConfigError) Error() string {
	return fmt.Sprintf("missing configuration: %s", e.missing)
}

func (e noConfigError) Is(target error) bool {
	_, ok := target.(noConfigError)
	return ok
}

// newHostAgentInfo assembles a HostAgentInfo message.
func newHostAgentInfo(ctx context.Context, c serviceData) (info *landscapeapi.HostAgentInfo, err error) {
	defer decorate.OnError(&err, "could not assemble HostAgentInfo message")

	conf, err := newLandscapeHostConf(c.config())
	if err != nil {
		return info, err
	}

	distros := c.database().GetAll()
	var instances []*landscapeapi.HostAgentInfo_InstanceInfo
	for _, d := range distros {
		instanceInfo, err := newInstanceInfo(d)

		if errors.As(err, &newInstanceInfoMinorError{}) {
			log.Warningf(ctx, "Landscape: skipping distro %q from landscape info: %v", d.Name(), err)
			continue
		}

		if err != nil {
			log.Errorf(ctx, "Landscape:  skipping distro %q from landscape info: %v", d.Name(), err)
			continue
		}

		instances = append(instances, instanceInfo)
	}

	uid, err := c.config().LandscapeAgentUID()
	if err != nil {
		return info, err
	}

	info = &landscapeapi.HostAgentInfo{
		Token:       conf.ubuntuProToken,
		Uid:         uid,
		Hostname:    c.hostname(),
		Instances:   instances,
		AccountName: conf.accountName,
	}

	// Optional arguments
	if conf.registrationKey != "" {
		info.RegistrationKey = &conf.registrationKey
	}

	if defaultDistro, ok, err := gowsl.DefaultDistro(ctx); err != nil {
		log.Warningf(ctx, "Landscape: could not get default distro: %v", err)
		return info, nil
	} else if ok {
		n := defaultDistro.Name()
		info.DefaultInstanceId = &n
	}

	return info, nil
}

// transportCredentials reads the Landscape client config to check if a SSL public key is specified.
//
// If this credential is not specified, an insecure credential is returned.
// If the credential is specified but erroneous, an error is returned.
func transportCredentials(sslPublicKeyPath string) (cred credentials.TransportCredentials, err error) {
	defer decorate.OnError(&err, "Landscape credentials")

	if sslPublicKeyPath == "" {
		return insecure.NewCredentials(), nil
	}

	cert, err := os.ReadFile(sslPublicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("could not load SSL public key file: %v", err)
	}

	certPool := x509.NewCertPool()
	if ok := certPool.AppendCertsFromPEM(cert); !ok {
		return nil, fmt.Errorf("failed to add server CA's certificate: %v", err)
	}

	return credentials.NewTLS(&tls.Config{
		RootCAs:    certPool,
		MinVersion: tls.VersionTLS12,
	}), nil
}

// newLandscapeHostConf extracts the information relevant to the agent from the LandscapeConfig
// configuration data.
// Any missing necessary value will result in a noConfigError.
// Any missing optional value will be set to a default value.
func newLandscapeHostConf(config Config) (conf landscapeHostConf, err error) {
	defer decorate.OnError(&err, "could not extract Windows settings from the config")

	conf.ubuntuProToken, _, err = config.Subscription()
	if err != nil {
		return conf, err
	} else if conf.ubuntuProToken == "" {
		return landscapeHostConf{}, noConfigError{missing: "Ubuntu Pro token"}
	}

	out, _, err := config.LandscapeClientConfig()
	if err != nil {
		return conf, fmt.Errorf("could not obtain Landscape client config: %v", err)
	}

	if out == "" {
		// No Landscape config: return defaults
		return landscapeHostConf{}, noConfigError{missing: "Landscape configuration"}
	}

	ini, err := ini.Load(strings.NewReader(out))
	if err != nil {
		return conf, fmt.Errorf("could not parse Landscape client config: %v", err)
	}

	// Note: all these functions only return errors when the section/key does not exist.

	sec, err := ini.GetSection("client")
	if err == nil {
		k, err := sec.GetKey("ssl_public_key")
		if err == nil {
			conf.sslPublicKey = k.String()
		}

		k, err = sec.GetKey("account_name")
		if err == nil {
			conf.accountName = k.String()
		}

		k, err = sec.GetKey("registration_key")
		if err == nil {
			conf.registrationKey = k.String()
		}
	}

	sec, err = ini.GetSection("host")
	if err != nil {
		return landscapeHostConf{}, noConfigError{missing: "Host URL"}
	}

	urlKey, err := sec.GetKey("url")
	if err != nil {
		return landscapeHostConf{}, noConfigError{missing: "Host URL"}
	}
	conf.hostagentURL = urlKey.String()

	return conf, nil
}

type newInstanceInfoMinorError struct {
	err error
}

func (e newInstanceInfoMinorError) Error() string {
	return e.err.Error()
}

// newInstanceInfo initializes a Instances_InstanceInfo from a distro.
func newInstanceInfo(d *distro.Distro) (info *landscapeapi.HostAgentInfo_InstanceInfo, err error) {
	state, err := d.State()
	if err != nil {
		return info, err
	}

	var instanceState landscapeapi.InstanceState
	switch state {
	case gowsl.Running:
		instanceState = landscapeapi.InstanceState_Running
	case gowsl.Stopped:
		instanceState = landscapeapi.InstanceState_Stopped
	case gowsl.Installing, gowsl.NonRegistered, gowsl.Uninstalling:
		return nil, newInstanceInfoMinorError{err: fmt.Errorf("cannot query distro due to its state: %s", state)}
	default:
		return nil, fmt.Errorf("unknown state %q", state)
	}

	properties := d.Properties()
	info = &landscapeapi.HostAgentInfo_InstanceInfo{
		Id:            d.Name(),
		Name:          properties.Hostname,
		VersionId:     properties.VersionID,
		InstanceState: instanceState,
	}

	return info, nil
}

func distributeConfig(ctx context.Context, db *database.DistroDB, landscapeConf string) {
	var err error
	for _, distro := range db.GetAll() {
		t := tasks.LandscapeConfigure{
			Config: landscapeConf,
		}
		err = errors.Join(err, distro.SubmitTasks(t))
	}

	if err != nil {
		log.Warningf(ctx, "Landscape: could not submit configuration tasks: %v", err)
	}
}

type retryConnection struct {
	once sync.Once
	ch   chan struct{}
	mu   sync.RWMutex
}

func newRetryConnection() *retryConnection {
	var r retryConnection
	r.init()
	return &r
}

func (r *retryConnection) init() {
	r.ch = make(chan struct{})
	r.once = sync.Once{}
}

func (r *retryConnection) Stop() {
	r.Request()
}

func (r *retryConnection) Request() {
	r.mu.RLock()
	defer r.mu.RUnlock()

	r.once.Do(func() { close(r.ch) })
}

func (r *retryConnection) Await() <-chan struct{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.ch
}

func (r *retryConnection) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.once.Do(func() { close(r.ch) })
	r.init()
}
