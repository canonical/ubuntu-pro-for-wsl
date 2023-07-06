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
	"github.com/ubuntu/decorate"
)

const (
	registryPath = `Software\Canonical\UbuntuPro`

	defaultToken = ""

	fieldLandscapeURL   = "LandscapeURL"
	defaultLandscapeURL = "www.example.com"
)

// fieldsProToken contains the fields in the registry where each source will store its token.
var fieldsProToken = map[SubscriptionSource]string{
	SubscriptionOrganization:   "ProTokenOrg",
	SubscriptionUser:           "ProTokenUser",
	SubscriptionMicrosoftStore: "ProTokenStore",
}

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
	proTokens map[SubscriptionSource]string
	data      configData

	registry Registry

	mu *sync.RWMutex
}

// configData is a bag of data unrelated to the subscription status.
type configData struct {
	landscapeURL string
}

// SubscriptionSource indicates the method the subscription was acquired.
type SubscriptionSource int

// Subscription types. Sorted in ascending order of precedence.
const (
	// SubscriptionNone -> no subscription.
	SubscriptionNone SubscriptionSource = iota

	// SubscriptionManual -> the subscription was obtained by introducing a pro token
	// via the registry by the sys admin.
	SubscriptionOrganization

	// SubscriptionUser -> the subscription was obtained by introducing a pro token
	// via the registry or the GUI.
	SubscriptionUser

	// SubscriptionMicrosoftStore -> the subscription was acquired via the Microsoft Store.
	SubscriptionMicrosoftStore

	// subscriptionMaxPriority is a sentinel value to make looping simpler.
	// It must always be the last value in the enum.
	subscriptionMaxPriority
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
		registry:  opts.registry,
		mu:        &sync.RWMutex{},
		proTokens: make(map[SubscriptionSource]string),
	}

	return m
}

// ProToken returns the Pro Token associated to the current subscription.
// If there is no active subscription, an empty string is returned.
func (c *Config) ProToken(ctx context.Context) (string, error) {
	token, _, err := c.Subscription(ctx)
	if err != nil {
		return "", err
	}

	return token, nil
}

// Subscription returns the ProToken and the method it was acquired with (if any).
func (c *Config) Subscription(ctx context.Context) (token string, source SubscriptionSource, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.load(ctx); err != nil {
		return "", SubscriptionNone, fmt.Errorf("could not load: %v", err)
	}

	for src := subscriptionMaxPriority - 1; src > SubscriptionNone; src-- {
		token, ok := c.proTokens[src]
		if !ok {
			continue
		}

		if token == "" {
			continue
		}

		return token, src, nil
	}

	return "", SubscriptionNone, nil
}

// IsReadOnly returns whether the registry can be written to.
func (c *Config) IsReadOnly() (b bool, err error) {
	// CreateKey is equivalent to OpenKey if the key already existed
	k, err := c.registry.HKCUCreateKey(registryPath, registry.WRITE)
	if errors.Is(err, registry.ErrAccessDenied) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("could not open registry key: %w", err)
	}

	c.registry.CloseKey(k)
	return false, nil
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
func (c *Config) SetSubscription(ctx context.Context, proToken string, source SubscriptionSource) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Load before dumping to avoid overriding recent changes to registry
	if err := c.load(ctx); err != nil {
		return err
	}

	old := c.proTokens[source]
	c.proTokens[source] = proToken

	if err := c.dump(); err != nil {
		log.Errorf(ctx, "Could not update subscription in registry, token will be ignored: %v", err)
		c.proTokens[source] = old
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

func (c *Config) load(ctx context.Context) (err error) {
	defer decorate.OnError(&err, "could not load data for Config")

	// Read registry
	proTokens, data, err := c.loadRegistry(ctx)
	if err != nil {
		return err
	}

	// Commit to loaded data
	c.proTokens = proTokens
	c.data = data

	return nil
}

func (c *Config) loadRegistry(ctx context.Context) (proTokens map[SubscriptionSource]string, data configData, err error) {
	defer decorate.OnError(&err, "could not load from registry")

	proTokens = map[SubscriptionSource]string{}

	k, err := c.registry.HKCUOpenKey(registryPath, registry.READ)
	if errors.Is(err, registry.ErrKeyNotExist) {
		log.Debug(ctx, "Registry key does not exist, using default values")
		data.landscapeURL = defaultLandscapeURL
		return proTokens, data, nil
	}
	if err != nil {
		return proTokens, data, err
	}
	defer c.registry.CloseKey(k)

	for source, field := range fieldsProToken {
		proToken, e := c.readValue(ctx, k, field, defaultToken)
		if e != nil {
			err = errors.Join(err, fmt.Errorf("could not read %q: %v", field, e))
			continue
		}

		if proToken == "" {
			continue
		}

		proTokens[source] = proToken
	}

	if err != nil {
		return nil, data, err
	}

	data.landscapeURL, err = c.readValue(ctx, k, fieldLandscapeURL, defaultLandscapeURL)
	if err != nil {
		return proTokens, data, err
	}

	return proTokens, data, nil
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

func (c *Config) dump() (err error) {
	defer decorate.OnError(&err, "could not store Config data")

	// CreateKey is equivalent to OpenKey if the key already existed
	k, err := c.registry.HKCUCreateKey(registryPath, registry.WRITE)
	if err != nil {
		return fmt.Errorf("could not open or create registry key: %w", err)
	}
	defer c.registry.CloseKey(k)

	for source, field := range fieldsProToken {
		err := c.registry.WriteValue(k, field, c.proTokens[source])
		if err != nil {
			return fmt.Errorf("could not write into registry key: %w", err)
		}
	}

	if err := c.registry.WriteValue(k, fieldLandscapeURL, c.data.landscapeURL); err != nil {
		return fmt.Errorf("could not write into registry key: %v", err)
	}

	return nil
}
