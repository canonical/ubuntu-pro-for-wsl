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
	defaultToken  = ""

	fieldLandscapeURL   = "LandscapeURL"
	defaultLandscapeURL = "www.example.com"
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
	subscription Subscription
	data         configData

	registry  Registry

	mu *sync.RWMutex
}

// configData is a bag of data unrelated to the subscription status.
type configData struct {
	landscapeURL string
}

// Subscription contains the pro token and some metadata.
type Subscription struct {
	ProToken string
	Source   SubscriptionSource
}

// SubscriptionSource indicates the method the subscription was acquired.
type SubscriptionSource int

const (
	// SubscriptionNone -> no subscription.
	SubscriptionNone SubscriptionSource = iota

	// SubscriptionManual -> the subscription was obtained by introducing a pro token
	// via the registry or the GUI.
	SubscriptionManual

	// SubscriptionMicrosoftStore -> the subscription was acquired via the Microsoft Store.
	SubscriptionMicrosoftStore
)

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

// ProToken returns the Pro Token associated to the current subscription.
// If there is no active subscription, an empty string is returned.
func (c *Config) ProToken(ctx context.Context) (string, error) {
	s, err := c.Subscription(ctx)
	if err != nil {
		return "", err
	}

	return s.ProToken, nil
}

// Subscription returns the ProToken and the method it was acquired with (if any).
func (c *Config) Subscription(ctx context.Context) (s Subscription, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.load(ctx); err != nil {
		return s, fmt.Errorf("could not load: %v", err)
	}

	return c.subscription, nil
}

// ProvisioningTasks returns a slice of all tasks to be submitted upon first contact with a distro.
func (c *Config) ProvisioningTasks(ctx context.Context) ([]task.Task, error) {
	token, err := c.ProToken(ctx)
	if err != nil {
		return nil, err
	}

	return []task.Task{tasks.ProAttachment{Token: token}}, nil
}

// SetSubscription overwrites the value of the pro token and the method with which it has been acquired.
func (c *Config) SetSubscription(ctx context.Context, subscription Subscription) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var old Subscription
	old, c.subscription = c.subscription, subscription

	if err := c.dump(); err != nil {
		log.Errorf(ctx, "Could not write token into registry, token will be ignored: %v", err)
		c.subscription = old
		return err
	}

	return nil
}

// LandscapeURL returns the value of the landscape server URL.
func (c *Config) LandscapeURL(ctx context.Context) (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if err := c.load(ctx); err != nil {
		return "", fmt.Errorf("could not load: %v", err)
	}

	return c.data.landscapeURL, nil
}

func (c *Config) load(ctx context.Context) error {
	k, err := c.registry.HKCUOpenKey(registryPath, registry.READ)
	if errors.Is(err, registry.ErrKeyNotExist) {
		log.Debug(ctx, "Registry key does not exist, using default values")
		proToken = defaultToken
		data.landscapeURL = defaultLandscapeURL
		return proToken, data, nil
	}
	if err != nil {
		return proToken, data, err
	}
	defer c.registry.CloseKey(k)

	proToken, err = c.readValue(ctx, k, fieldProToken, defaultToken)
	if err != nil {
		return proToken, data, err
	}

	data.landscapeURL, err = c.readValue(ctx, k, fieldLandscapeURL, defaultLandscapeURL)
	if err != nil {
		return proToken, data, err
	}

	return proToken, data, nil
}

func (c *Config) readValue(ctx context.Context, key uintptr, field string, defaultValue string) (string, error) {
	value, err := c.registry.ReadValue(key, field)
	if errors.Is(err, registry.ErrFieldNotExist) {
		log.Debugf(ctx, "Registry value %q does not exist, defaulting to %q", field, defaultValue)
		return defaultValue, nil
	}
	if err != nil {
		return "", err
	}
	return value, nil
}

func (c *Config) dump() error {
	// CreateKey is equivalent to OpenKey if the key already existed
	k, err := c.registry.HKCUCreateKey(registryPath, registry.WRITE)
	if err != nil {
		return fmt.Errorf("could not open or create registry key: %w", err)
	}
	defer c.registry.CloseKey(k)

	if err := c.registry.WriteValue(k, fieldProToken, c.subscription.ProToken); err != nil {
		return fmt.Errorf("could not write into registry key: %w", err)
	}

	if err := c.registry.WriteValue(k, fieldLandscapeURL, c.data.landscapeURL); err != nil {
		return fmt.Errorf("could not write into registry key: %v", err)
	}

	return nil
}
