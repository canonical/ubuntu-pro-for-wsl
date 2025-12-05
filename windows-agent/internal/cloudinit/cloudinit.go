// Package cloudinit has some helpers to set up cloud-init configuration.
package cloudinit

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	log "github.com/canonical/ubuntu-pro-for-wsl/common/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/config"
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
	dataDir string
	conf    Config
}

// New creates a CloudInit object and attaches it to the configuration notifier.
func New(ctx context.Context, conf Config, publicDir string) (CloudInit, error) {
	c := CloudInit{
		dataDir: filepath.Join(publicDir, ".cloud-init"),
		conf:    conf,
	}

	// c.writeAgentData() is no longer guaranteed to create the cloud-init directory,
	// so let's check now if we have permission to do so.
	if err := os.MkdirAll(c.dataDir, 0700); err != nil {
		return CloudInit{}, fmt.Errorf("could not create cloud-init directory: %v", err)
	}

	if err := c.writeAgentData(); err != nil {
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
		return removeFileInDir(c.dataDir, "agent.yaml")
	}

	err = writeFileInDir(c.dataDir, "agent.yaml", cloudInit)
	if err != nil {
		return err
	}

	return nil
}

// metadata is a struct that serializes the instance ID as yaml.
type metadata struct {
	InstanceID string `yaml:"instance-id"`
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
		if err = writeFileInDir(c.dataDir, distroName+".meta-data", md); err != nil {
			return fmt.Errorf("could not create instance metadata file: %v", err)
		}
	}

	if err := writeFileInDir(c.dataDir, distroName+".user-data", []byte(cloudInit)); err != nil {
		return fmt.Errorf("could not create distro-specific cloud-init file: %v", err)
	}

	return nil
}

// removeFileInDir attempts to remove the file 'dir/file' if it exists. Missing file is not an error.
func removeFileInDir(dir, file string) error {
	err := os.Remove(filepath.Join(dir, file))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// writeFileInDir:
// 1. Creates the directory if it did not exist.
// 2. Creates the file using the temp-then-move pattern. This avoids read/write races.
func writeFileInDir(dir string, file string, contents []byte) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("could not create directory: %v", err)
	}

	path := filepath.Join(dir, file)
	tmp := path + ".tmp"

	if err := os.WriteFile(tmp, contents, 0600); err != nil {
		return fmt.Errorf("could not write: %v", err)
	}

	if err := os.Rename(tmp, path); err != nil {
		if r := os.Remove(tmp); r != nil {
			log.Warningf(context.Background(), "could not remove temporary file: %v", r)
		}
		return err // Error message already says 'cannot rename'
	}

	return nil
}

// RemoveDistroData removes cloud-init user data to be used for a distro in particular.
//
// No error is returned if the data did not exist.
func (c CloudInit) RemoveDistroData(distroName string) (err error) {
	defer decorate.OnError(&err, "could not remove distro-specific cloud-init file")

	path := filepath.Join(c.dataDir, distroName+".user-data")

	err = os.Remove(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	return nil
}

func marshalConfig(conf Config) ([]byte, error) {
	contents := make(map[string]interface{})

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

func ubuntuProModule(c Config, out map[string]interface{}) error {
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

func landscapeModule(c Config, out map[string]interface{}) error {
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
