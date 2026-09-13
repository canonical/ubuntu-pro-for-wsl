// Package cloudinit has some helpers to set up cloud-init configuration.
package cloudinit

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	log "github.com/canonical/ubuntu-pro-for-wsl/common/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/config"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/securefiles"
	"github.com/ubuntu/decorate"
	"go.yaml.in/yaml/v3"
	"gopkg.in/ini.v1"
)

// Config is a configuration provider for ProToken and the Landscape config.
type Config interface {
	Subscription() (string, config.Source, error)
	LandscapeClientConfig() (string, config.Source, error)
}

// CloudInit contains necessary data to drop cloud-init user data files for WSL's data source to pick them up.
type CloudInit struct {
	dir  *securefiles.Custodian
	conf Config
}

// New creates a CloudInit object and attaches it to the configuration notifier.
// The custodian must already be scoped to the cloud-init sub-tree; the writer only addresses files by leaf name.
func New(ctx context.Context, conf Config, dir *securefiles.Custodian) (CloudInit, error) {
	c := CloudInit{
		dir:  dir,
		conf: conf,
	}

	// Purge every node that does not carry the agent's watermark and regenerate the agent's own
	// cloud-init file. Stamped nodes are simply left alone.
	if err := c.startupPurge(ctx); err != nil {
		return CloudInit{}, err
	}

	return c, nil
}

// Update is syntax sugar to call writeAgentData and log any error.
func (c CloudInit) Update(ctx context.Context) {
	if err := c.writeAgentData(); err != nil {
		log.Warningf(ctx, "Cloud-init: %v", err)
	}
}

// writeAgentData writes the agent's cloud-init data file.
func (c CloudInit) writeAgentData() (err error) {
	defer decorate.OnError(&err, "could not create agent's cloud-init file")

	cloudInit, err := marshalConfig(c.conf)
	if err != nil {
		return err
	}

	// Nothing to write, we don't want an empty agent.yaml confusing the real cloud-init.
	if cloudInit == nil {
		err := c.dir.Remove("agent.yaml")
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}

	return c.dir.WriteFile("agent.yaml", cloudInit)
}

// metadata is a struct that serializes the instance ID as yaml.
type metadata struct {
	InstanceID string `yaml:"instance-id"`
}

// startupPurge removes every node in the sub-tree that does not carry the agent's watermark and
// regenerates the agent's own data file from the current configuration. Stamped nodes are left
// untouched: cloud-init data is consumed exactly once, at the instance's first boot, so there is
// nothing to gain from reading it back and rewriting it. The watermark is the whole adoption
// policy: an unstamped node is foreign by definition, whatever its name.
func (c CloudInit) startupPurge(ctx context.Context) error {
	isOurs := func(rel string) bool {
		// Directories are never adopted: this sub-tree legitimately holds files only.
		if _, err := c.dir.ReadDir(rel); err == nil {
			return false
		}
		owned, err := c.dir.IsOwned(rel)
		if err != nil {
			log.Warningf(ctx, "cloud-init: could not check ownership of %q: %v", rel, err)
			return false
		}
		return owned
	}

	removed, err := c.dir.Purge(isOurs)
	if err != nil {
		return fmt.Errorf("could not purge cloud-init sub-tree: %v", err)
	}

	for _, rel := range removed {
		if strings.HasPrefix(rel, ".tmp-") || rel == "agent.yaml" {
			// Leftover temporaries and the agent's own file are expected churn.
			continue
		}
		if strings.HasSuffix(rel, ".user-data") || strings.HasSuffix(rel, ".meta-data") {
			// A node named like per-distro data but not stamped by us smells like tampering.
			log.Errorf(ctx, "cloud-init: removed per-distro node %q that is not stamped as agent-owned", rel)
			continue
		}
		log.Warningf(ctx, "cloud-init: removed unrecognised node from sub-tree: %s", rel)
	}

	return c.writeAgentData()
}

// WriteDistroData writes cloud-init data to be used for a particular distro instance.
func (c CloudInit) WriteDistroData(distroName string, cloudInit string, instanceID string) error {
	// Handle the metadata first. It would be otherwise annoying if this data would be supposed
	// to initialize a new instance per Landscape request and everything else worked but the
	// request ID didn't come through, the server would never tie the new instance to the
	// installation activity.
	if instanceID != "" {
		md, err := yaml.Marshal(metadata{InstanceID: instanceID})
		if err != nil {
			return fmt.Errorf("could not marshal metadata: %v", err)
		}
		if err := c.dir.WriteFile(distroName+".meta-data", md); err != nil {
			return fmt.Errorf("could not create instance metadata file: %v", err)
		}
	}

	if err := c.dir.WriteFile(distroName+".user-data", []byte(cloudInit)); err != nil {
		return fmt.Errorf("could not create distro-specific cloud-init file: %v", err)
	}

	return nil
}

// RemoveDistroData removes cloud-init user data to be used for a distro in particular.
//
// No error is returned if the data did not exist.
func (c CloudInit) RemoveDistroData(distroName string) (err error) {
	defer decorate.OnError(&err, "could not remove distro-specific cloud-init file")

	if err := c.dir.Remove(distroName + ".user-data"); errors.Is(err, fs.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	return nil
}

func marshalConfig(conf Config) ([]byte, error) {
	contents := make(map[string]any)

	if err := ubuntuProModule(conf, contents); err != nil {
		return nil, err
	}

	if err := landscapeModule(conf, contents); err != nil {
		return nil, err
	}

	// If there is no config to write, then let's not write an empty object with comments to avoid confusing cloud-init.
	if len(contents) == 0 {
		return nil, nil
	}

	out, err := yaml.Marshal(contents)
	if err != nil {
		return nil, fmt.Errorf("could not Marshal user data as a YAML: %v", err)
	}

	w := &bytes.Buffer{}

	if _, err := fmt.Fprintln(w, "#cloud-config\n# This file was generated automatically and must not be edited"); err != nil {
		return nil, fmt.Errorf("could not write #cloud-config stenza and warning message: %v", err)
	}

	if _, err := w.Write(out); err != nil {
		return nil, fmt.Errorf("could not write config body: %v", err)
	}

	return w.Bytes(), nil
}

func ubuntuProModule(c Config, out map[string]any) error {
	token, src, err := c.Subscription()
	if err != nil {
		return err
	}
	if src == config.SourceNone {
		return nil
	}

	type uaModule struct {
		Token string `yaml:"token"`
	}

	out["ubuntu_pro"] = uaModule{Token: token}
	return nil
}

func landscapeModule(c Config, out map[string]any) error {
	conf, src, err := c.LandscapeClientConfig()
	if err != nil {
		return err
	}
	if src == config.SourceNone {
		return nil
	}

	var landscapeModule struct {
		Client map[string]string `yaml:"client"`
	}

	f, err := ini.Load(strings.NewReader(conf))
	if err != nil {
		return fmt.Errorf("could not load Landscape configuration file")
	}

	section, err := f.GetSection("client")
	if err != nil {
		return nil // Empty section
	}

	landscapeModule.Client = make(map[string]string)
	for _, keyName := range section.KeyStrings() {
		landscapeModule.Client[keyName] = section.Key(keyName).String()
	}

	// Enforce a deferred registration with Landscape.
	landscapeModule.Client["no_start"] = ""
	landscapeModule.Client["skip_registration"] = ""

	// Add a placeholder computer title to prevent cloud-init schema warnings.
	landscapeModule.Client["computer_title"] = "wsl"

	out["landscape"] = landscapeModule
	return nil
}
