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

	if err := c.loadFile(); err != nil {
		return fmt.Errorf("could not load config from the chache file: %v", err)
	}

	if err := c.loadRegistry(); err != nil {
		return fmt.Errorf("could not load config from the registry: %v", err)
	}

	return nil
}

func (c *Config) loadFile() (err error) {
	out, err := os.ReadFile(c.cachePath)
	if errors.Is(err, fs.ErrNotExist) {
		out = []byte{}
	} else if err != nil {
		return fmt.Errorf("could not read cache file: %v", err)
	}

	h := marshalHelper{
		Landscape:    c.landscape,
		Subscription: c.subscription,
	}

	if err := yaml.Unmarshal(out, &h); err != nil {
		return fmt.Errorf("could not umarshal cache file: %v", err)
	}

	c.landscape = h.Landscape
	c.subscription = h.Subscription

	return nil
}

func (c *Config) loadRegistry() (err error) {
	k, err := c.registry.HKCUOpenKey(registryPath, registry.READ)
	if errors.Is(err, registry.ErrKeyNotExist) {
		// Default values
		c.subscription.Organization = ""
		c.landscape.OrgConfig = ""
		return nil
	}
	if err != nil {
		return err
	}
	defer c.registry.CloseKey(k)

	proToken, err := readFromRegistry(c.registry, k, "UbuntuProToken")
	if err != nil {
		return err
	}

	config, err := readFromRegistry(c.registry, k, "LandscapeConfig")
	if err != nil {
		return err
	}

	c.subscription.Organization = proToken
	c.landscape.OrgConfig = config

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

func (c *Config) dump() (err error) {
	defer decorate.OnError(&err, "could not store Config data")

	// Backup the file in case the registry write fails.
	// This avoids partial writes
	restore, err := makeBackup(c.cachePath)
	if err != nil {
		return err
	}

	if err := c.dumpFile(); err != nil {
		return err
	}

	if err := c.dumpRegistry(); err != nil {
		return errors.Join(err, restore())
	}

	return nil
}

// makeBackup makes a backup of the selected file. It returns
// a restoring function.
func makeBackup(originalPath string) (func() error, error) {
	backupPath := originalPath + ".backup"

	err := os.Rename(originalPath, backupPath)
	if errors.Is(err, fs.ErrNotExist) {
		// File does not exist. Restoring means deleting it.
		return func() error {
			return os.RemoveAll(originalPath)
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("could not create backup: %v", err)
	}

	// File does exist. Restoring means moving it back.
	return func() error {
		return os.Rename(backupPath, originalPath)
	}, nil
}

func (c *Config) dumpRegistry() error {
	// CreateKey is equivalent to OpenKey if the key already existed
	k, err := c.registry.HKCUCreateKey(registryPath, registry.WRITE)
	if err != nil {
		return fmt.Errorf("could not open or create registry key: %w", err)
	}
	defer c.registry.CloseKey(k)

	if err := c.registry.WriteValue(k, "UbuntuProToken", c.subscription.Organization); err != nil {
		return fmt.Errorf("could not write UbuntuProToken into registry key: %v", err)
	}

	if err := c.registry.WriteMultilineValue(k, "LandscapeConfig", c.landscape.OrgConfig); err != nil {
		return fmt.Errorf("could not write LandscapeConfig into registry key: %v", err)
	}

	return nil
}

func (c *Config) dumpFile() error {
	h := marshalHelper{
		Landscape:    c.landscape,
		Subscription: c.subscription,
	}

	out, err := yaml.Marshal(h)
	if err != nil {
		return fmt.Errorf("could not marshal config: %v", err)
	}

	if err := os.WriteFile(c.cachePath, out, 0600); err != nil {
		return fmt.Errorf("could not write config cache file: %v", err)
	}

	return nil
}
