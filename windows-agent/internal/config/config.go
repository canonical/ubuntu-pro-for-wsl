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

	"github.com/canonical/ubuntu-pro-for-windows/common"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/contracts"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/database"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/task"
	log "github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/tasks"
	"github.com/ubuntu/decorate"
)

// Config manages configuration parameters. It is a wrapper around a dictionary
// that reads and updates the config file.
type Config struct {
	// data
	subscription subscription
	landscape    landscapeConf

	// disk backing
	cachePath string

	// Sync
	mu *sync.Mutex

	// observers are called after any configuration changes.
	observers []func()
}

// New creates and initializes a new Config object.
func New(ctx context.Context, cachePath string) (m *Config) {
	m = &Config{
		cachePath: filepath.Join(cachePath, "config"),
		mu:        &sync.Mutex{},
	}

	return m
}

// Notify appends a callback. It'll be called every time any configuration changes.
func (c *Config) Notify(f func()) {
	c.observers = append(c.observers, f)
}

func (c *Config) notifyObservers() {
	for _, f := range c.observers {
		// This needs to be in a goroutine because notifyObservers is sometimes
		// called under the config mutex. The callback trying to grab the mutex
		// (to read the config) would cause a deadlock otherwise.
		go f()
	}
}

// Subscription returns the ProToken and the method it was acquired with (if any).
func (c *Config) Subscription(ctx context.Context) (token string, source Source, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.load(); err != nil {
		return "", SourceNone, fmt.Errorf("could not load: %v", err)
	}

	token, source = c.subscription.resolve()
	return token, source, nil
}

// ProvisioningTasks returns a slice of all tasks to be submitted upon first contact with a distro.
func (c *Config) ProvisioningTasks(ctx context.Context, distroName string) ([]task.Task, error) {
	var taskList []task.Task

	// Refresh data from registry
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.load(); err != nil {
		return nil, fmt.Errorf("could not load: %v", err)
	}

	// Ubuntu Pro attachment
	proToken, _ := c.subscription.resolve()
	taskList = append(taskList, tasks.ProAttachment{Token: proToken})

	// Landscape config
	lconf, _ := c.landscape.resolve()
	taskList = append(taskList, tasks.LandscapeConfigure{Config: lconf, HostagentUID: c.landscape.UID})

	return taskList, nil
}

// LandscapeClientConfig returns the value of the landscape server URL and
// the method it was acquired with (if any).
func (c *Config) LandscapeClientConfig(ctx context.Context) (string, Source, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.load(); err != nil {
		return "", SourceNone, fmt.Errorf("could not load: %v", err)
	}

	conf, src := c.landscape.resolve()
	return conf, src, nil
}

// SetUserSubscription overwrites the value of the user-provided Ubuntu Pro token.
func (c *Config) SetUserSubscription(ctx context.Context, proToken string) error {
	if _, src := c.subscription.resolve(); src > SourceUser {
		return errors.New("attempted to set a user subscription when there already is a higher priority one")
	}

	return c.set(ctx, &c.subscription.User, proToken)
}

// setStoreSubscription overwrites the value of the store-provided Ubuntu Pro token.
func (c *Config) setStoreSubscription(ctx context.Context, proToken string) error {
	if _, src := c.subscription.resolve(); src > SourceMicrosoftStore {
		return errors.New("attempted to set a store subscription when there already is a higher priority one")
	}

	return c.set(ctx, &c.subscription.Store, proToken)
}

// SetLandscapeAgentUID overrides the Landscape agent UID.
func (c *Config) SetLandscapeAgentUID(ctx context.Context, uid string) error {
	return c.set(ctx, &c.landscape.UID, uid)
}

// set is a generic method to safely modify the config.
func (c *Config) set(ctx context.Context, field *string, value string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Load before dumping to avoid overriding recent changes to file
	if err := c.load(); err != nil {
		return err
	}

	old := *field
	*field = value

	c.notifyObservers()

	if err := c.dump(); err != nil {
		log.Errorf(ctx, "Could not update settings: %v", err)
		*field = old
		return err
	}

	return nil
}

// LandscapeAgentUID returns the UID assigned to this agent by the Landscape server.
// An empty string is returned if no UID has been assigned.
func (c *Config) LandscapeAgentUID(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.load(); err != nil {
		return "", fmt.Errorf("could not load: %v", err)
	}

	return c.landscape.UID, nil
}

// FetchMicrosoftStoreSubscription contacts Ubuntu Pro's contract server and the Microsoft Store
// to check if the user has an active subscription that provides a pro token. If so, that token is used.
func (c *Config) FetchMicrosoftStoreSubscription(ctx context.Context, args ...contracts.Option) (err error) {
	defer decorate.OnError(&err, "could not validate subscription against Microsoft Store")

	_, src, err := c.Subscription(ctx)
	if err != nil {
		return fmt.Errorf("could not get current subscription status: %v", err)
	}

	// Shortcut to avoid spamming the contract server
	// We don't need to request a new token if we have a non-expired one
	if src == SourceMicrosoftStore {
		valid, err := contracts.ValidSubscription(args...)
		if err != nil {
			return fmt.Errorf("could not obtain current subscription status: %v", err)
		}

		if valid {
			log.Debug(ctx, "Microsoft Store subscription is active")
			return nil
		}

		log.Debug(ctx, "No valid Microsoft Store subscription")
	}

	proToken, err := contracts.NewProToken(ctx, args...)
	if err != nil {
		return fmt.Errorf("could not get ProToken from Microsoft Store: %v", err)
	}

	if proToken != "" {
		log.Debugf(ctx, "Obtained Ubuntu Pro token from the Microsoft Store: %q", common.Obfuscate(proToken))
	}

	if err := c.setStoreSubscription(ctx, proToken); err != nil {
		return err
	}

	return nil
}

// RegistryData contains the data that the Ubuntu Pro registry key can provide.
type RegistryData struct {
	UbuntuProToken, LandscapeConfig string
}

// UpdateRegistryData takes in data from the registry and applies it as necessary.
func (c *Config) UpdateRegistryData(ctx context.Context, data RegistryData, db *database.DistroDB) error {
	taskList, err := c.collectRegistrySettingsTasks(ctx, data, db)
	if err != nil {
		return err
	}

	// Apply tasks for updated settings
	for _, d := range db.GetAll() {
		err = errors.Join(err, d.SubmitDeferredTasks(taskList...))
	}

	if err != nil {
		return fmt.Errorf("could not submit new task to certain distros: %v", err)
	}

	return nil
}

// collectRegistrySettingsTasks looks at the registry data to see if any of them have changed since this
// function was last called. It returns a list of tasks to run triggered by these changes, and updates
// the config.
func (c *Config) collectRegistrySettingsTasks(ctx context.Context, data RegistryData, db *database.DistroDB) ([]task.Task, error) {
	type getTask = func(*Config, context.Context, RegistryData, *database.DistroDB) (task.Task, error)

	c.mu.Lock()
	defer c.mu.Unlock()

	// Load up-to-date state
	if err := c.load(); err != nil {
		return nil, err
	}

	// Collect tasks for updated settings
	var errs error
	var taskList []task.Task
	for _, f := range []getTask{(*Config).getTaskOnNewSubscription, (*Config).getTaskOnNewLandscape} {
		task, err := f(c, ctx, data, db)
		if err != nil {
			errs = errors.Join(errs, err)
			continue
		}
		if task != nil {
			taskList = append(taskList, task)
		}
	}

	if errs != nil {
		log.Warningf(ctx, "Could not obtain some updated registry settings: %v", errs)
	}

	c.notifyObservers()

	// Dump updated checksums
	if err := c.dump(); err != nil {
		return nil, fmt.Errorf("could not store updated registry settings: %v", err)
	}

	return taskList, nil
}

// getTaskOnNewSubscription checks if the subscription has changed since the last time it was called. If so, the new subscription
// is returned in the form of a task.
func (c *Config) getTaskOnNewSubscription(ctx context.Context, data RegistryData, db *database.DistroDB) (task.Task, error) {
	c.subscription.Organization = data.UbuntuProToken

	if !hasChanged(data.UbuntuProToken, &c.subscription.Checksum) {
		return nil, nil
	}
	log.Debug(ctx, "New organization-provided Ubuntu Pro subscription settings detected in registry")

	proToken, _ := c.subscription.resolve()
	return tasks.ProAttachment{Token: proToken}, nil
}

// getTaskOnNewLandscape checks if the Landscape settings has changed since the last time it was called. If so, the
// new Landscape settings are returned in the form of a task.
func (c *Config) getTaskOnNewLandscape(ctx context.Context, data RegistryData, db *database.DistroDB) (task.Task, error) {
	c.landscape.OrgConfig = data.LandscapeConfig

	// We append them just so we can compute a combined checksum
	serialized := fmt.Sprintf("%s%s", data.LandscapeConfig, c.landscape.UID)
	if !hasChanged(serialized, &c.landscape.Checksum) {
		return nil, nil
	}

	log.Debug(ctx, "New Landscape settings detected in registry")

	lconf, _ := c.landscape.resolve()
	return tasks.LandscapeConfigure{Config: lconf, HostagentUID: c.landscape.UID}, nil
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
