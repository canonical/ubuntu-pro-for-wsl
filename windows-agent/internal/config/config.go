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
	"strings"
	"sync"

	log "github.com/canonical/ubuntu-pro-for-wsl/common/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/database"
	"github.com/ubuntu/decorate"
	"gopkg.in/ini.v1"
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
	notifyLandscape LandscapeNotifier
	notifyUbuntuPro UbuntuProNotifier
}

// UbuntuProNotifier is a function that is called when the Ubuntu Pro subscription changes.
type UbuntuProNotifier func(ctx context.Context, token string)

// LandscapeNotifier is a function that is called when the Landscape configuration changes.
type LandscapeNotifier func(ctx context.Context, config, uid string)

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
		notifyLandscape: func(ctx context.Context, config, uid string) {},
	}

	return m
}

// SetLandscapeNotifier sets the function to be called when the Landscape configuration changes.
func (c *Config) SetLandscapeNotifier(notify LandscapeNotifier) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.notifyLandscape = notify
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

// LandscapeClientConfig returns the complete Landscape client configuration and
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

// SetUserLandscapeConfig overwrites the value of the user-provided Landscape configuration.
func (c *Config) SetUserLandscapeConfig(ctx context.Context, landscapeConfig string) error {
	if _, src := c.Landscape.resolve(); src > SourceUser {
		return errors.New("attempted to set a user-provided landscape configuration when there already is a higher priority one")
	}

	landscapeConfig, err := completeLandscapeConfig(landscapeConfig, c.Landscape.UID)
	if err != nil {
		return fmt.Errorf("config: could not complete Landscape configuration: %v", err)
	}

	isNew, err := c.set(&c.Landscape.UserConfig, landscapeConfig)
	if err != nil {
		return errors.New("config: could not set Landscape configuration")
	}

	if isNew {
		c.notifyLandscape(ctx, landscapeConfig, c.Landscape.UID)
	}

	return nil
}

// SetLandscapeAgentUID overrides the Landscape agent UID and notify listeners.
func (c *Config) SetLandscapeAgentUID(ctx context.Context, uid string) error {
	isUpdated, conf, err := c.setAgentUIDAndUpdateClientConf(ctx, uid)
	if err != nil {
		return err
	}

	if isUpdated {
		log.Debugf(ctx, "config: notifying Landscape config listeners about agent UID change: %s", uid)
		c.notifyLandscape(ctx, conf, uid)
	}

	return nil
}

// setAgentUIDAndUpdateClientConf sets the agent UID and updates the client configuration, returning the new configuration and the 'true' boolean flag to indicate that the configuration has changed,
// allowing the caller to trigger notifications, preventing that from happening while we hold a lock.
func (c *Config) setAgentUIDAndUpdateClientConf(ctx context.Context, uid string) (bool, string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.load(); err != nil {
		return false, "", fmt.Errorf("config: could not set Landscape agent UID: %v", err)
	}

	if c.Landscape.UID == uid {
		log.Info(ctx, "config: no changes in the agent UID")
		return false, "", nil
	}

	c.Landscape.UID = uid

	// From this point on we have something to dump, but since `c.dump()` affects the values returned from this function, let's add yet one more indirection,
	// setting the values we want to return first, and then deciding based on the result of `c.dump()`
	mustNotify, updatedConf, err := func() (bool, string, error) {
		landscapeConf, src := c.Landscape.resolve()
		if src == SourceNone {
			log.Info(ctx, "config: no client configuration to notify about agent UID change")
			return false, "", nil
		}

		updated, err := completeLandscapeConfig(landscapeConf, uid)
		if err != nil {
			return false, "", fmt.Errorf("config: could not update client conf with agent UID changes: %v", err)
		}

		switch src {
		case SourceUser:
			c.Landscape.UserConfig = updated
		case SourceRegistry:
			c.Landscape.OrgConfig = updated
		default:
			return false, "", fmt.Errorf("config: could not update client conf with agent UID changes: unexpected source for client configuration: %v", src)
		}
		return true, landscapeConf, nil
	}()

	if e := c.dump(); e != nil {
		return false, "", fmt.Errorf("config: could not set Landscape agent UID: %v", e)
	}
	return mustNotify, updatedConf, err
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
	// 4. Landscape drops connection, and reconnects

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

	if err = c.load(); err != nil {
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
	conf, err := completeLandscapeConfig(data.LandscapeConfig, c.Landscape.UID)
	if err != nil {
		log.Errorf(ctx, "Config: removing Landscape configuration from registry: %v", err)
	}
	if hasChanged(conf, &c.Landscape.Checksum) {
		log.Debug(ctx, "Config: new Landscape configuration received from the registry")
		c.Landscape.OrgConfig = conf

		// We must resolve the landscape config in case a lower priority config becomes active
		resolv, _ := c.Landscape.resolve()
		afterUnlock = append(afterUnlock, func() {
			c.notifyLandscape(ctx, resolv, c.Landscape.UID)
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

// completeLandscapeConfig completes the Landscape configuration by adding the hostagent_uid field to the client section,
// making it ready for consumption by the Landscape client inside the distro instances.
func completeLandscapeConfig(landscapeConf, hostAgentUID string) (string, error) {
	if landscapeConf == "" {
		return "", nil
	}
	conf, err := ini.Load(strings.NewReader(landscapeConf))
	if err != nil {
		return "", fmt.Errorf("could not parse Landscape configuration: %v", err)
	}

	clientSection, err := conf.GetSection("client")
	if err != nil {
		return "", fmt.Errorf("could not find the client section in the Landscape configuration: %v", err)
	}

	if hostAgentUID == "" {
		return landscapeConf, nil
	}

	keyName := "hostagent_uid"
	if key, err := clientSection.GetKey(keyName); err == nil {
		key.SetValue(hostAgentUID)
	} else {
		_, err = clientSection.NewKey(keyName, hostAgentUID)
		if err != nil {
			return "", fmt.Errorf("could not add the %s key to the client section: %v", keyName, err)
		}
	}

	// Write the ini to a string
	var b strings.Builder

	if _, err = conf.WriteTo(&b); err != nil {
		return "", fmt.Errorf("could not output the modified configuration: %v", err)
	}

	return b.String(), nil
}
