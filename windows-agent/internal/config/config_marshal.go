package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/config/registry"
	"github.com/ubuntu/decorate"
	"gopkg.in/yaml.v3"
)

type marshalHelper struct {
	Landscape    landscapeConf
	Subscription subscription
}

func (c *Config) load() (err error) {
	defer decorate.OnError(&err, "could not load data for Config")

	var h marshalHelper

	if err := h.loadFile(c.cachePath); err != nil {
		return fmt.Errorf("could not load config from the cache file: %v", err)
	}

	if err := h.loadRegistry(c.registry); err != nil {
		return fmt.Errorf("could not load config from the registry: %v", err)
	}

	c.landscape = h.Landscape
	c.subscription = h.Subscription

	return nil
}

func (h *marshalHelper) loadFile(cachePath string) (err error) {
	out, err := os.ReadFile(cachePath)
	if errors.Is(err, fs.ErrNotExist) {
		out = []byte{}
	} else if err != nil {
		return fmt.Errorf("could not read cache file: %v", err)
	}

	if err := yaml.Unmarshal(out, h); err != nil {
		return fmt.Errorf("could not umarshal cache file: %v", err)
	}

	return nil
}

func (h *marshalHelper) loadRegistry(reg Registry) (err error) {
	k, err := reg.HKCUOpenKey(registryPath, registry.READ)
	if errors.Is(err, registry.ErrKeyNotExist) {
		// Default values
		h.Subscription.Organization = ""
		h.Landscape.OrgConfig = ""
		return nil
	}
	if err != nil {
		return err
	}
	defer reg.CloseKey(k)

	proToken, err := readFromRegistry(reg, k, "UbuntuProToken")
	if err != nil {
		return err
	}

	config, err := readFromRegistry(reg, k, "LandscapeConfig")
	if err != nil {
		return err
	}

	h.Subscription.Organization = proToken
	h.Landscape.OrgConfig = config

	return nil
}

func readFromRegistry(r Registry, key uintptr, field string) (string, error) {
	value, err := r.ReadValue(key, field)
	if errors.Is(err, registry.ErrFieldNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("could not read %q from registry", field)
	}

	return value, nil
}

func (c Config) dump() (err error) {
	defer decorate.OnError(&err, "could not store Config data")

	h := marshalHelper{
		Landscape:    c.landscape,
		Subscription: c.subscription,
	}

	if err := h.dumpFile(c.cachePath); err != nil {
		return err
	}

	return nil
}

func (h marshalHelper) dumpFile(cachePath string) error {
	out, err := yaml.Marshal(h)
	if err != nil {
		return fmt.Errorf("could not marshal config: %v", err)
	}

	if err := os.WriteFile(cachePath, out, 0600); err != nil {
		return fmt.Errorf("could not write config cache file: %v", err)
	}

	return nil
}
