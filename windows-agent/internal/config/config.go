// Package config manages configuration parameters. It manages the configuration for
// the Windows Agent so that only a single config file needs to exist.
package config

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/config/registry"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/task"
	log "github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/tasks"
)

const (
	registryPath = `Software\Canonical\UbuntuPro`

	fieldProToken = "ProToken"
)

// Registry abstracts away access to the windows registry.
type Registry interface {
	HKCUCreateKey(path string, access uint32) (newk uintptr, err error)
	HKCUOpenKey(path string, access uint32) (uintptr, error)
	CloseKey(k uintptr)
	ReadValue(k uintptr, field string) (value string, err error)
	WriteValue(k uintptr, field string, value string) (err error)
}

// Config manages configuration parameters. It is a wrapper around a dictionary
// that reads and updates the config file.
type Config struct {
	proToken string
	registry Registry
	mu       *sync.RWMutex
}

type options struct {
	registry Registry
}

// Option is an optional argument for New.
type Option func(*options)

// WithRegistry allows for overriding the windows registry with a mock.
func WithRegistry(r Registry) Option {
	return func(o *options) {
		o.registry = r
	}
}

// New creates and initializes a new Config object.
func New(ctx context.Context, args ...Option) (m *Config) {
	var opts options

	for _, f := range args {
		f(&opts)
	}

	if opts.registry == nil {
		opts.registry = registry.Windows{}
	}

	m = &Config{
		registry: opts.registry,
		mu:       &sync.RWMutex{},
	}

	return m
}

// ProToken returns the value of the pro token.
func (c *Config) ProToken(ctx context.Context) (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if err := c.load(ctx); err != nil {
		return c.proToken, fmt.Errorf("could not load: %v", err)
	}

	return c.proToken, nil
}

// ProvisioningTasks returns a slice of all tasks to be submitted upon first contact with a distro.
func (c *Config) ProvisioningTasks(ctx context.Context) ([]task.Task, error) {
	token, err := c.ProToken(ctx)
	if err != nil {
		return nil, err
	}

	return []task.Task{tasks.ProAttachment{Token: token}}, nil
}

// SetProToken overwrites the value of the pro token.
func (c *Config) SetProToken(ctx context.Context, token string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var oldToken string
	oldToken, c.proToken = c.proToken, token

	if err := c.dump(); err != nil {
		log.Errorf(ctx, "Could not write token into registry, token will be ignored: %v", err)
		c.proToken = oldToken
		return err
	}

	return nil
}

func (c *Config) load(ctx context.Context) error {
	k, err := c.registry.HKCUOpenKey(registryPath, registry.READ)
	if errors.Is(err, registry.ErrKeyNotExist) {
		log.Debug(ctx, "Registry key does not exist, defaulting to empty token")
		c.proToken = ""
		return nil
	}
	if err != nil {
		return err
	}
	defer c.registry.CloseKey(k)

	token, err := c.registry.ReadValue(k, fieldProToken)
	if errors.Is(err, registry.ErrFieldNotExist) {
		log.Debugf(ctx, "Registry value %s does not exist, defaulting to empty token", fieldProToken)
		c.proToken = ""
		return nil
	}
	if err != nil {
		return err
	}

	c.proToken = token

	return nil
}

func (c *Config) dump() error {
	// CreateKey is equivalent to OpenKey if the key already existed
	k, err := c.registry.HKCUCreateKey(registryPath, registry.WRITE)
	if err != nil {
		return fmt.Errorf("could not open or create registry key: %w", err)
	}
	defer c.registry.CloseKey(k)

	if err := c.registry.WriteValue(k, fieldProToken, c.proToken); err != nil {
		return fmt.Errorf("could not write into registry key: %w", err)
	}

	return nil
}
