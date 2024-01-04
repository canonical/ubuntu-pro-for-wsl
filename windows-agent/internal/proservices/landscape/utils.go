package landscape

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"strings"

	landscapeapi "github.com/canonical/landscape-hostagent-api"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/distro"
	log "github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/grpc/logstreamer"
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
}

// newHostAgentInfo assembles a HostAgentInfo message.
func newHostAgentInfo(ctx context.Context, c serviceData) (info *landscapeapi.HostAgentInfo, err error) {
	token, _, err := c.config().Subscription(ctx)
	if err != nil {
		return info, err
	}

	conf, err := readLandscapeHostConf(ctx, c.config())
	if err != nil {
		return info, fmt.Errorf("could not read config: %v", err)
	}

	distros := c.database().GetAll()
	var instances []*landscapeapi.HostAgentInfo_InstanceInfo
	for _, d := range distros {
		instanceInfo, err := newInstanceInfo(d)

		if errors.As(err, &newInstanceInfoMinorError{}) {
			log.Warningf(ctx, "Skipping from landscape info: %v", err)
			continue
		}

		if err != nil {
			log.Errorf(ctx, "Skipping from landscape info: %v", err)
			continue
		}

		instances = append(instances, instanceInfo)
	}

	uid, err := c.config().LandscapeAgentUID(ctx)
	if err != nil {
		return info, err
	}

	info = &landscapeapi.HostAgentInfo{
		Token:       token,
		Uid:         uid,
		Hostname:    c.hostname(),
		Instances:   instances,
		AccountName: conf.accountName,
	}

	if conf.registrationKey != "" {
		info.RegistrationKey = &conf.registrationKey
	}

	return info, nil
}

// transportCredentials reads the Landscape client config to check if a SSL public key is specified.
//
// If this credential is not specified, an insecure credential is returned.
// If the credential is specified but erroneous, an error is returned.
func (conf landscapeHostConf) transportCredentials() (cred credentials.TransportCredentials, err error) {
	defer decorate.OnError(&err, "Landscape credentials")

	if conf.sslPublicKey == "" {
		return insecure.NewCredentials(), nil
	}

	cert, err := os.ReadFile(conf.sslPublicKey)
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

func readLandscapeHostConf(ctx context.Context, config Config) (landscapeHostConf, error) {
	conf := landscapeHostConf{
		// TODO: default-initialize the hostagentURL to Canonical's SaaS.
	}

	out, _, err := config.LandscapeClientConfig(ctx)
	if err != nil {
		return conf, fmt.Errorf("could not obtain Landscape config: %v", err)
	}

	if out == "" {
		// No Landscape config: return defaults
		return conf, nil
	}

	ini, err := ini.Load(strings.NewReader(out))
	if err != nil {
		return conf, fmt.Errorf("could not parse Landscape config file: %v", err)
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
	if err == nil {
		k, err := sec.GetKey("url")
		if err == nil {
			conf.hostagentURL = k.String()
		}
	}

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
		return nil, newInstanceInfoMinorError{err: fmt.Errorf("distro %q is in state %q. Only %q and %q are accepted", d.Name(), state, gowsl.Running, gowsl.Stopped)}
	default:
		return nil, fmt.Errorf("distro %q is in unknown state %q", d.Name(), state)
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
