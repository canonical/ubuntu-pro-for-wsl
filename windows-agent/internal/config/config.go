// Package config manages configuration parameters. It manages the configuration for
// the Windows Agent so that only a single config file needs to exist.
package config

import (
	"context"
	"crypto/sha512"
	"encoding/base64"
	"errors"
	"fmt"
	"path/filepath"
	"sync"

	log "github.com/canonical/ubuntu-pro-for-wsl/common/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/database"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/task"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/tasks"
	"github.com/ubuntu/decorate"
)

// Config manages configuration parameters. It is a wrapper around a dictionary
// that reads and updates the config file.
type Config struct {
	// data
	configState

	// disk backing
	storagePath string

	// Sync
	mu *sync.Mutex

	// observers are notified after any configuration changes.
	notifyLandsape  LandscapeNotifier
	notifyUbuntuPro UbuntuProNotifier
}

// UbuntuProNotifier is a function that is called when the Ubuntu Pro subscription changes.
type UbuntuProNotifier func(ctx context.Context, token string)

// LandscapeNotifier is a function that is called when the Landscape configuration changes.
type LandscapeNotifier func(ctx context.Context, config string, uid string)

// configState contains the actual configuration data.
//
// Its methods must be public for proper YAML (un)marshalling.
type configState struct {
	Subscription subscription
	Landscape    landscapeConf
}

// New creates and initializes a new Config object.
func New(ctx context.Context, cachePath string) (m *Config) {
	m = &Config{
		storagePath: filepath.Join(cachePath, "config"),
		mu:          &sync.Mutex{},

		// No-ops to avoid nil checks
		notifyUbuntuPro: func(ctx context.Context, token string) {},
		notifyLandsape:  func(ctx context.Context, config string, uid string) {},
	}

	return m
}

// SetLandscapeNotifier sets the function to be called when the Landscape configuration changes.
func (c *Config) SetLandscapeNotifier(notify LandscapeNotifier) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.notifyLandsape = notify
}

// SetUbuntuProNotifier sets the function to be called when the Ubuntu Pro subscription changes.
func (c *Config) SetUbuntuProNotifier(notify UbuntuProNotifier) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.notifyUbuntuPro = notify
}

// Subscription returns the ProToken and the method it was acquired with (if any).
func (c *Config) Subscription() (token string, source Source, err error) {
	s, err := c.get()
	if err != nil {
		return "", SourceNone, fmt.Errorf("config: could not get Ubuntu Pro subscription: %v", err)
	}

	token, source = s.Subscription.resolve()
	return token, source, nil
}

// ProvisioningTasks returns a slice of all tasks to be submitted upon first contact with a distro.
func (c *Config) ProvisioningTasks(ctx context.Context, distroName string) ([]task.Task, error) {
	var taskList []task.Task

	// Refresh data from registry
	s, err := c.get()
	if err != nil {
		return nil, fmt.Errorf("config: could not get provisioning tasks: %v", err)
	}

	// Ubuntu Pro attachment
	proToken, _ := s.Subscription.resolve()
	taskList = append(taskList, tasks.ProAttachment{Token: proToken})

	// Landscape config
	lconf, _ := s.Landscape.resolve()
	taskList = append(taskList, tasks.LandscapeConfigure{Config: lconf, HostagentUID: s.Landscape.UID})

	return taskList, nil
}

// LandscapeClientConfig returns the value of the landscape server URL and
// the method it was acquired with (if any).
func (c *Config) LandscapeClientConfig() (string, Source, error) {
	s, err := c.get()
	if err != nil {
		return "", SourceNone, fmt.Errorf("config: could not get Landscape configuration: %v", err)
	}

	conf, src := s.Landscape.resolve()
	return conf, src, nil
}

// SetUserSubscription overwrites the value of the user-provided Ubuntu Pro token.
func (c *Config) SetUserSubscription(ctx context.Context, proToken string) (err error) {
	defer decorate.OnError(&err, "config: could not set user-provided Ubuntu Pro subscription")

	s, err := c.get()
	if err != nil {
		return fmt.Errorf("could not get exiting Ubuntu Pro subscription: %v", err)
	}

	if _, src := s.Subscription.resolve(); src > SourceUser {
		return errors.New("higher priority subscription active")
	}

	isNew, err := c.set(&c.configState.Subscription.User, proToken)
	if err != nil {
		return err
	}

	if isNew {
		c.notifyUbuntuPro(ctx, proToken)
	}

	return nil
}

// SetStoreSubscription overwrites the value of the store-provided Ubuntu Pro token.
func (c *Config) SetStoreSubscription(ctx context.Context, proToken string) (err error) {
	defer decorate.OnError(&err, "could not set Microsoft-Store-provided Ubuntu Pro subscription")

	s, err := c.get()
	if err != nil {
		return fmt.Errorf("could not get exiting Ubuntu Pro subscription: %v", err)
	}

	if _, src := s.Subscription.resolve(); src > SourceMicrosoftStore {
		return errors.New("higher priority subscription active")
	}

	isNew, err := c.set(&c.configState.Subscription.Store, proToken)
	if err != nil {
		return err
	}

	if isNew {
		c.notifyUbuntuPro(ctx, proToken)
	}

	return nil
}

// SetLandscapeAgentUID overrides the Landscape agent UID.
func (c *Config) SetLandscapeAgentUID(uid string) error {
	if _, err := c.set(&c.Landscape.UID, uid); err != nil {
		return fmt.Errorf("config: could not set Landscape agent UID: %v", err)
	}

	return nil
}

func (c *Config) get() (s configState, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.load(); err != nil {
		return s, err
	}

	return c.configState, nil
}

// set is a generic method to safely modify the config.
func (c *Config) set(field *string, value string) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.load(); err != nil {
		return false, err
	}

	old := *field
	if old == value {
		return false, nil
	}

	*field = value

	if err := c.dump(); err != nil {
		*field = old
		return false, err
	}

	return true, nil
}

// LandscapeAgentUID returns the UID assigned to this agent by the Landscape server.
// An empty string is returned if no UID has been assigned.
func (c *Config) LandscapeAgentUID() (string, error) {
	s, err := c.get()
	if err != nil {
		return "", fmt.Errorf("config: could not get Landscape agent UID: %v", err)
	}

	// We do not notify Landscape to avoid a potential infinite loop:
	// 1. Start connection
	// 2. Get UID
	// 3. Notify Landscape
	// 4. Landcape drops connection, and reconnects

	return s.Landscape.UID, nil
}

// RegistryData contains the data that the Ubuntu Pro registry key can provide.
type RegistryData struct {
	UbuntuProToken, LandscapeConfig string
}

// UpdateRegistryData takes in data from the registry and applies it as necessary.
func (c *Config) UpdateRegistryData(ctx context.Context, data RegistryData, db *database.DistroDB) (err error) {
	defer decorate.OnError(&err, "config: could not update registry-provided data")

	// We must perform the notification outside the lock to avoid deadlocks
	afterUnlock := []func(){}
	defer func() {
		for _, f := range afterUnlock {
			f()
		}
	}()

	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.load(); err != nil {
		return err
	}

	// Ubuntu Pro subscription
	c.configState.Subscription.Organization = data.UbuntuProToken
	if hasChanged(data.UbuntuProToken, &c.configState.Subscription.Checksum) {
		log.Debug(ctx, "Config: new Ubuntu Pro subscription received from the registry")

		// We must resolve the subscription in case a lower priority token becomes active
		resolv, _ := c.configState.Subscription.resolve()
		afterUnlock = append(afterUnlock, func() {
			c.notifyUbuntuPro(ctx, resolv)
		})
	}

	// Landscape configuration
	c.Landscape.OrgConfig = data.LandscapeConfig
	checksumInput := data.LandscapeConfig + c.Landscape.UID
	if hasChanged(checksumInput, &c.Landscape.Checksum) {
		log.Debug(ctx, "Config: new Landscape configuration received from the registry")

		// We must resolve the landscape config in case a lower priority config becomes active
		resolv, _ := c.Landscape.resolve()
		afterUnlock = append(afterUnlock, func() {
			c.notifyLandsape(ctx, resolv, c.Landscape.UID)
		})
	}

	if err := c.dump(); err != nil {
		return err
	}

	return nil
}

// hasChanged detects if the current value is different from the last time it was used.
// If the value has changed, the checksum will be updated.
func hasChanged(newValue string, checksum *string) bool {
	var newCheckSum string
	if len(newValue) != 0 {
		raw := sha512.Sum512([]byte(newValue))
		newCheckSum = base64.StdEncoding.EncodeToString(raw[:])
	}

	if *checksum == newCheckSum {
		return false
	}

	*checksum = newCheckSum
	return true
}
