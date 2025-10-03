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
	"time"

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
			log.Warningf(ctx, "Landscape: skipping distro %q from Landscape info: %v", d.Name(), err)
			continue
		}

		if err != nil {
			log.Errorf(ctx, "Landscape:  skipping distro %q from landscape info: %v", d.Name(), err)
			continue
		}

		instances = append(instances, instanceInfo)
	}

	var unmanaged []*landscapeapi.HostAgentInfo_InstanceInfo
	un := c.database().GetUnmanagedDistros()
	for _, b := range un {
		instanceInfo, err := newInstanceInfoFromBasicInfo(b)

		if errors.As(err, &newInstanceInfoMinorError{}) {
			log.Warningf(ctx, "Landscape: skipping unmanaged distro %q from Landscape info: %v", b.Name, err)
			continue
		}

		if err != nil {
			log.Errorf(ctx, "Landscape: skipping unmanaged distro %q from Landscape info: %v", b.Name, err)
			continue
		}

		unmanaged = append(unmanaged, instanceInfo)
	}

	uid, err := c.config().LandscapeAgentUID()
	if err != nil {
		return info, err
	}

	info = &landscapeapi.HostAgentInfo{
		Token:              conf.ubuntuProToken,
		Uid:                uid,
		Hostname:           c.hostname(),
		Instances:          instances,
		AccountName:        conf.accountName,
		UnmanagedInstances: unmanaged,
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

type transportCredentialsType struct{}

// InsecureCredentials is the key used in tests for insecure credentials.
var InsecureCredentials = transportCredentialsType{}

// transportCredentials reads the Landscape client config to check if a SSL public key is specified.
//
// If this credential is not specified, credentials based on the system's certificate pool is returned.
// If the SSL public key is specified but invalid, an error is returned.
// If the context has the "InsecureCredentials" key set to "true", insecure credentials are returned (for testing purposes).
func transportCredentials(ctx context.Context, sslPublicKeyPath string) (cred credentials.TransportCredentials, err error) {
	defer decorate.OnError(&err, "Landscape credentials")

	isInsecure := ctx.Value(InsecureCredentials)
	// ctx.Value() returns 'any', thus this comparison is cleaner than a type assertion.
	if isInsecure == true {
		log.Warningf(ctx, "Landscape: context requires insecure credentials, ignoring server's public key %s", sslPublicKeyPath)
		return insecure.NewCredentials(), nil
	}

	if sslPublicKeyPath == "" {
		certPool, err := x509.SystemCertPool()
		if err != nil {
			return nil, fmt.Errorf("could not load system certificates: %v", err)
		}

		return credentials.NewTLS(&tls.Config{
			RootCAs:    certPool,
			MinVersion: tls.VersionTLS12,
		}), nil
	}

	log.Infof(ctx, "Landscape: loading server's SSL public key %s", sslPublicKeyPath)
	cert, err := os.ReadFile(sslPublicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("could not load SSL public key file: %v", err)
	}

	certPool := x509.NewCertPool()
	if ok := certPool.AppendCertsFromPEM(cert); !ok {
		return nil, fmt.Errorf("failed to add server's certificate to the trust pool: %v", err)
	}

	log.Infof(ctx, "Landscape: using server's SSL public key %s instead of system's certificate pool", sslPublicKeyPath)
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
	state, err := tryDistroState(d, 1*time.Second, 5*time.Second)
	if err != nil {
		return info, err
	}

	instanceState, err := translateInstanceState(state)
	if err != nil {
		return nil, err
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

// newInstanceInfoFromBasicInfo initializes a Instances_InstanceInfo from a BasicDistroInfo.
func newInstanceInfoFromBasicInfo(b database.BasicDistroInfo) (info *landscapeapi.HostAgentInfo_InstanceInfo, err error) {
	instanceState, err := translateInstanceState(b.State)
	if err != nil {
		return nil, err
	}
	info = &landscapeapi.HostAgentInfo_InstanceInfo{
		Id:            b.Name,
		Name:          b.Hostname,
		VersionId:     b.VersionID,
		InstanceState: instanceState,
	}

	return info, nil
}

// translateInstanceState converts a gowsl.State into a landscapeapi.InstanceState to report to the
// Landscape server.
func translateInstanceState(state gowsl.State) (instanceState landscapeapi.InstanceState, err error) {
	switch state {
	case gowsl.Running:
		instanceState = landscapeapi.InstanceState_Running
	case gowsl.Stopped:
		instanceState = landscapeapi.InstanceState_Stopped
	case gowsl.Installing, gowsl.NonRegistered, gowsl.Uninstalling:
		return instanceState, fmt.Errorf("cannot query distro due to its state: %s", state)
	default:
		return instanceState, fmt.Errorf("unknown state %q", state)
	}

	return instanceState, nil
}

// tryDistroState attempts to get the state of a distro, retrying every interval until timeout.
func tryDistroState(d *distro.Distro, interval, timeout time.Duration) (state gowsl.State, err error) {
	// set the timeout timer
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	state, err = d.State()
	for err != nil {
		select {
		case <-time.After(interval):
			state, err = d.State()
		case <-timer.C:
			return state, fmt.Errorf("WSL internal error after retry: %v", err)
		}
	}

	return state, nil
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

// filterClientSection removes all sections from the Landscape configuration except the [client] section.
func filterClientSection(landscapeConf string) (string, error) {
	f, err := ini.Load(strings.NewReader(landscapeConf))
	if err != nil {
		return "", fmt.Errorf("could not load Landscape configuration: %v", err)
	}

	if !f.HasSection("client") {
		return "", errors.New("missing [client] section in Landscape configuration")
	}

	for _, section := range f.Sections() {
		if section.Name() != "client" {
			f.DeleteSection(section.Name())
		}
	}

	var b strings.Builder
	if _, err := f.WriteTo(&b); err != nil {
		return "", fmt.Errorf("could not write filtered configuration: %v", err)
	}

	return b.String(), nil
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
