package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/ubuntu/decorate"
	"gopkg.in/yaml.v3"
)

func (c *Config) load() (err error) {
	defer decorate.OnError(&err, "could not load config from disk")

	var s configState

	out, err := os.ReadFile(c.storagePath)
	if errors.Is(err, fs.ErrNotExist) {
		out = []byte{}
	} else if err != nil {
		return fmt.Errorf("could not read config file: %v", err)
	}

	if err := yaml.Unmarshal(out, &s); err != nil {
		return fmt.Errorf("could not umarshal config file: %v", err)
	}

	// Registry data must not be overridden
	tokenOrg := c.configState.Subscription.Organization
	landscapeOrg := c.Landscape.OrgConfig

	c.configState = s

	c.configState.Subscription.Organization = tokenOrg
	c.Landscape.OrgConfig = landscapeOrg

	return nil
}

func (c *Config) dump() (err error) {
	defer decorate.OnError(&err, "could not store config to disk")

	out, err := yaml.Marshal(c.configState)
	if err != nil {
		return fmt.Errorf("could not marshal config: %v", err)
	}

	if err := os.WriteFile(c.storagePath, out, 0600); err != nil {
		return fmt.Errorf("could not write config file: %v", err)
	}

	return nil
}
