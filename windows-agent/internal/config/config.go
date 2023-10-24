// Package config manages configuration parameters. It manages the configuration for
// the Windows Agent so that only a single config file needs to exist.
package config

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/config/registry"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/contracts"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/task"
	log "github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/tasks"
	"github.com/ubuntu/decorate"
)

const (
	registryPath = `Software\Canonical\UbuntuPro`

	defaultToken = ""

	fieldLandscapeClientConfig   = "LandscapeClientConfig"
	defaultLandscapeClientConfig = ""

	fieldLandscapeAgentURL   = "LandscapeAgentURL"
	defaultLandscapeAgentURL = ""
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
	WriteMultilineValue(k uintptr, field string, value string) (err error)
}

// Config manages configuration parameters. It is a wrapper around a dictionary
// that reads and updates the config file.
type Config struct {
	proTokens map[SubscriptionSource]string
	data      configData

	registry Registry

	mu *sync.Mutex
}

// configData is a bag of data unrelated to the subscription status.
type configData struct {
	landscapeClientConfig string
	landscapeAgentURL     string
}

// SubscriptionSource indicates the method the subscription was acquired.
type SubscriptionSource int

// Subscription types. Sorted in ascending order of precedence.
const (
	// SubscriptionNone -> no subscription.
	SubscriptionNone SubscriptionSource = iota

	// SubscriptionOrganization -> the subscription was obtained by introducing a pro token
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
		mu:        &sync.Mutex{},
		proTokens: make(map[SubscriptionSource]string),
	}

	return m
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
func (c *Config) ProvisioningTasks(ctx context.Context, distroName string) ([]task.Task, error) {
	token, _, err := c.Subscription(ctx)
	if err != nil {
		return nil, err
	}

	taskList := []task.Task{
		tasks.ProAttachment{Token: token},
	}

	if conf, err := c.LandscapeClientConfig(ctx); err != nil {
		log.Errorf(ctx, "Could not generate provisioning task LandscapeConfigure: %v", err)
	} else {
		landscape := tasks.LandscapeConfigure{Config: conf}
		taskList = append(taskList, landscape)
	}

	return taskList, nil
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

// LandscapeAgentURL returns the value of the landscape server URL.
func (c *Config) LandscapeAgentURL(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.load(ctx); err != nil {
		return "", fmt.Errorf("could not load: %v", err)
	}

	return c.data.landscapeAgentURL, nil
}

// LandscapeClientConfig returns the value of the landscape server URL.
func (c *Config) LandscapeClientConfig(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.load(ctx); err != nil {
		return "", fmt.Errorf("could not load: %v", err)
	}

	return c.data.landscapeClientConfig, nil
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

	proTokens = make(map[SubscriptionSource]string)

	k, err := c.registry.HKCUOpenKey(registryPath, registry.READ)
	if errors.Is(err, registry.ErrKeyNotExist) {
		log.Debug(ctx, "Registry key does not exist, using default values")
		data.landscapeAgentURL = defaultLandscapeAgentURL
		data.landscapeClientConfig = defaultLandscapeClientConfig
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

	data.landscapeAgentURL, err = c.readValue(ctx, k, fieldLandscapeAgentURL, defaultLandscapeAgentURL)
	if err != nil {
		return proTokens, data, err
	}

	data.landscapeClientConfig, err = c.readValue(ctx, k, fieldLandscapeClientConfig, defaultLandscapeClientConfig)
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

	if err := c.registry.WriteValue(k, fieldLandscapeAgentURL, c.data.landscapeAgentURL); err != nil {
		return fmt.Errorf("could not write into registry key: %v", err)
	}

	if err := c.registry.WriteMultilineValue(k, fieldLandscapeClientConfig, c.data.landscapeClientConfig); err != nil {
		return fmt.Errorf("could not write into registry key: %v", err)
	}

	return nil
}

// FetchMicrosoftStoreSubscription contacts Ubuntu Pro's contract server and the Microsoft Store
// to check if the user has an active subscription that provides a pro token. If so, that token is used.
func (c *Config) FetchMicrosoftStoreSubscription(ctx context.Context) (err error) {
	defer decorate.OnError(&err, "could not validate subscription against Microsoft Store")

	readOnly, err := c.IsReadOnly()
	if err != nil {
		return fmt.Errorf("could not detect if subscription is user-managed: %v", err)
	}

	if readOnly {
		// No need to contact the store because we cannot change the subscription
		return fmt.Errorf("subscription cannot be user-managed")
	}

	proToken, err := contracts.ProToken(ctx)
	if err != nil {
		return fmt.Errorf("could not get ProToken from Microsoft Store: %v", err)
	}

	if err := c.SetSubscription(ctx, proToken, SubscriptionMicrosoftStore); err != nil {
		return err
	}

	return nil
}
