package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/ubuntu/decorate"
	"gopkg.in/yaml.v3"
)

type marshalHelper struct {
	Landscape    landscapeConf
	Subscription subscription
}

func (c *Config) load() (err error) {
	defer decorate.OnError(&err, "could not load config from disk")

	var h marshalHelper

	out, err := os.ReadFile(c.storagePath)
	if errors.Is(err, fs.ErrNotExist) {
		out = []byte{}
	} else if err != nil {
		return fmt.Errorf("could not read config file: %v", err)
	}

	if err := yaml.Unmarshal(out, &h); err != nil {
		return fmt.Errorf("could not umarshal config file: %v", err)
	}

	// Registry data must not be overridden
	tokenOrg := c.subscription.Organization
	landscapeOrg := c.landscape.OrgConfig

	c.subscription = h.Subscription
	c.landscape = h.Landscape

	c.subscription.Organization = tokenOrg
	c.landscape.OrgConfig = landscapeOrg

	return nil
}

func (c Config) dump() (err error) {
	defer decorate.OnError(&err, "could not store config to disk")

	h := marshalHelper{
		Landscape:    c.landscape,
		Subscription: c.subscription,
	}

	out, err := yaml.Marshal(&h)
	if err != nil {
		return fmt.Errorf("could not marshal config: %v", err)
	}

	if err := os.WriteFile(c.storagePath, out, 0600); err != nil {
		return fmt.Errorf("could not write config file: %v", err)
	}

	return nil
}
