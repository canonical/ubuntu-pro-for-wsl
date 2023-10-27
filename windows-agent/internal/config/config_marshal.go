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

	// Backup the file in case the registry write fails.
	// This avoids partial writes
	restore, err := makeBackup(c.cachePath)
	if err != nil {
		return err
	}

	h := marshalHelper{
		Landscape:    c.landscape,
		Subscription: c.subscription,
	}

	if err := h.dumpFile(c.cachePath); err != nil {
		return err
	}

	if err := h.dumpRegistry(c.registry); err != nil {
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

func (h marshalHelper) dumpRegistry(reg Registry) error {
	// CreateKey is equivalent to OpenKey if the key already existed
	k, err := reg.HKCUCreateKey(registryPath, registry.WRITE)
	if err != nil {
		return fmt.Errorf("could not open or create registry key: %w", err)
	}
	defer reg.CloseKey(k)

	if err := reg.WriteValue(k, "UbuntuProToken", h.Subscription.Organization); err != nil {
		return fmt.Errorf("could not write UbuntuProToken into registry key: %v", err)
	}

	if err := reg.WriteMultilineValue(k, "LandscapeConfig", h.Landscape.OrgConfig); err != nil {
		return fmt.Errorf("could not write LandscapeConfig into registry key: %v", err)
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
