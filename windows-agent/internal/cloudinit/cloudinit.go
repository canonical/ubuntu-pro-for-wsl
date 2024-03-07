// Package cloudinit has some helpers to set up cloud-init configuration.
package cloudinit

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"text/template"

	log "github.com/canonical/ubuntu-pro-for-wsl/common/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/config"
	"github.com/ubuntu/decorate"
	"gopkg.in/ini.v1"
	"gopkg.in/yaml.v3"
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

	// user is a function to get the Windows user. It exists for easier dependency injection in tests.
	user func() (*user.User, error)
}

// defaultDistroUserData is a template for the default user data, used to generate DefaultDistroData.
//
//go:embed default-user-data.template
var defaultDistroUserData string

// New creates a CloudInit object and attaches it to the configuration notifier.
func New(ctx context.Context, conf Config, publicDir string) (CloudInit, error) {
	c := CloudInit{
		dataDir: filepath.Join(publicDir, ".cloud-init"),
		conf:    conf,
		user:    user.Current,
	}

	if err := c.WriteAgentData(); err != nil {
		return c, err
	}

	return c, nil
}

// Notify is syntax sugar to call WriteAgentData and log any error.
func (c CloudInit) Notify(ctx context.Context) {
	if err := c.WriteAgentData(); err != nil {
		log.Warningf(ctx, "Cloud-init: %v", err)
	}
}

// WriteAgentData writes the agent's cloud-init data file.
func (c CloudInit) WriteAgentData() (err error) {
	defer decorate.OnError(&err, "could not create distro-specific cloud-init file")

	cloudInit, err := marshalConfig(c.conf)
	if err != nil {
		return err
	}

	err = writeFileInDir(c.dataDir, "agent.yaml", cloudInit)
	if err != nil {
		return err
	}

	return nil
}

// WriteDistroData writes cloud-init user data to be used for a distro in particular.
func (c CloudInit) WriteDistroData(distroName string, cloudInit string) error {
	err := writeFileInDir(c.dataDir, distroName+".user-data", []byte(cloudInit))
	if err != nil {
		return fmt.Errorf("could not create distro-specific cloud-init file: %v", err)
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
		_ = os.Remove(tmp)
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
	}
	if err != nil {
		return err
	}
	return nil
}

// DefaultDistroData generates the default cloud-init user data for the distro.
// It returns user data to create a user and set it as default.
func (c CloudInit) DefaultDistroData(ctx context.Context) (string, error) {
	userData, err := c.user()
	if err != nil {
		log.Warningf(ctx, "Cloud-init: using default username because we could not get Windows user: %v", err)

		userData = &user.User{
			Username: "ubuntu",
			Name:     "Ubuntu",
		}
	}

	t, err := template.New("cloud-init").Parse(defaultDistroUserData)
	if err != nil {
		return "", fmt.Errorf("could not parse default user data template: %v", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, userData); err != nil {
		return "", fmt.Errorf("could not execute default user data template: %v", err)
	}

	return buf.String(), nil
}

func marshalConfig(conf Config) ([]byte, error) {
	w := &bytes.Buffer{}

	if _, err := fmt.Fprintln(w, "# cloud-init"); err != nil {
		return nil, fmt.Errorf("could not write # cloud-init stenza: %v", err)
	}

	if _, err := fmt.Fprintln(w, "# This file was generated automatically and must not be edited"); err != nil {
		return nil, fmt.Errorf("could not write warning message: %v", err)
	}

	contents := make(map[string]interface{})

	if err := ubuntuAdvantageModule(conf, contents); err != nil {
		return nil, err
	}

	if err := landscapeModule(conf, contents); err != nil {
		return nil, err
	}

	out, err := yaml.Marshal(contents)
	if err != nil {
		return nil, fmt.Errorf("could not Marshal user data as a YAML: %v", err)
	}

	if _, err := w.Write(out); err != nil {
		return nil, fmt.Errorf("could not write config body: %v", err)
	}

	return w.Bytes(), nil
}

func ubuntuAdvantageModule(c Config, out map[string]interface{}) error {
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

	out["ubuntu_advantage"] = uaModule{Token: token}
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

	var landcapeModule struct {
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

	landcapeModule.Client = make(map[string]string)
	for _, keyName := range section.KeyStrings() {
		landcapeModule.Client[keyName] = section.Key(keyName).String()
	}

	out["landscape"] = landcapeModule
	return nil
}
