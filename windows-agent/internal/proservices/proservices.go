// Package proservices is in charge of managing the GRPC services and all business-logic side.
package proservices

import (
	"context"
	"crypto/sha512"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	agent_api "github.com/canonical/ubuntu-pro-for-windows/agentapi/go"
	"github.com/canonical/ubuntu-pro-for-windows/common"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/config"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/database"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/task"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/grpc/interceptorschain"
	log "github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/proservices/landscape"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/proservices/ui"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/proservices/wslinstance"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/tasks"
	"golang.org/x/exp/slices"
	"google.golang.org/grpc"
)

// Manager is the orchestrator of GRPC API services and business logic.
type Manager struct {
	uiService          ui.Service
	wslInstanceService wslinstance.Service
	landscapeService   *landscape.Client
	db                 *database.DistroDB
}

// options are the configurable functional options for the daemon.
type options struct {
	cacheDir string
	registry config.Registry
}

// Option is the function signature we are passing to tweak the daemon creation.
type Option func(*options)

// WithCacheDir overrides the cache directory used in the daemon.
func WithCacheDir(cachedir string) func(o *options) {
	return func(o *options) {
		if cachedir != "" {
			o.cacheDir = cachedir
		}
	}
}

// WithRegistry allows overriding the Windows registry with a different back-end.
func WithRegistry(registry config.Registry) func(o *options) {
	return func(o *options) {
		o.registry = registry
	}
}

// New returns a new GRPC services manager.
// It instantiates both ui and wsl instance services.
//
// Once done, Stop must be called to deallocate resources.
func New(ctx context.Context, args ...Option) (s Manager, err error) {
	log.Debug(ctx, "Building new GRPC services manager")

	// Apply given options.
	var opts options
	for _, f := range args {
		f(&opts)
	}

	if opts.cacheDir == "" {
		// Set default cache dir.
		appData := os.Getenv("LocalAppData")
		if appData == "" {
			return s, errors.New("Could not read env variable LocalAppData")
		}

		opts.cacheDir = filepath.Join(appData, common.LocalAppDataDir)
	}

	log.Debugf(ctx, "Manager service cache directory: %s", opts.cacheDir)

	if err := os.MkdirAll(opts.cacheDir, 0750); err != nil {
		return s, err
	}

	conf := config.New(ctx, config.WithRegistry(opts.registry))
	if err := conf.FetchMicrosoftStoreSubscription(ctx); err != nil {
		log.Warningf(ctx, "%v", err)
	}

	db, err := database.New(ctx, opts.cacheDir, conf)
	if err != nil {
		return s, err
	}
	defer func() {
		if err != nil {
			db.Close(ctx)
		}
	}()

	go func() {
		err := updateRegistrySettings(ctx, opts.cacheDir, conf, db)
		if err != nil {
			log.Warningf(ctx, "Could not update subscriptions: %v", err)
		}
	}()

	uiService := ui.New(ctx, conf, db)

	landscape, err := landscape.NewClient(conf, db)
	if err != nil {
		return s, err
	}

	if err := landscape.Connect(ctx); err != nil {
		log.Warningf(ctx, err.Error())
	}

	wslInstanceService, err := wslinstance.New(ctx, db, landscape)
	if err != nil {
		return s, err
	}

	return Manager{
		uiService:          uiService,
		wslInstanceService: wslInstanceService,
		db:                 db,
		landscapeService:   landscape,
	}, nil
}

// Stop deallocates resources in the services.
func (m Manager) Stop(ctx context.Context) {
	m.landscapeService.Stop(ctx)
	m.db.Close(ctx)
}

// RegisterGRPCServices returns a new grpc Server with the 2 api services attached to it.
// It also gets the correct middlewares hooked in.
func (m Manager) RegisterGRPCServices(ctx context.Context) *grpc.Server {
	log.Debug(ctx, "Registering GRPC services")

	grpcServer := grpc.NewServer(grpc.StreamInterceptor(
		interceptorschain.StreamServer(
		/*log.StreamServerInterceptor(logrus.StandardLogger()),
		logconnections.StreamServerInterceptor(),*/
		)))
	agent_api.RegisterUIServer(grpcServer, &m.uiService)
	agent_api.RegisterWSLInstanceServer(grpcServer, &m.wslInstanceService)

	return grpcServer
}

// updateRegistrySettings checks if any of the registry settings have changed since this function was last called.
// If so, updated settings are pushed to the distros.
func updateRegistrySettings(ctx context.Context, cacheDir string, conf *config.Config, db *database.DistroDB) error {
	type getTask = func(context.Context, string, *config.Config, *database.DistroDB) (task.Task, error)

	// Collect tasks for updated settings
	var acc error
	var taskList []task.Task
	for _, f := range []getTask{getNewSubscription} {
		task, err := f(ctx, cacheDir, conf, db)
		if err != nil {
			errors.Join(acc, err)
			continue
		}
		if task != nil {
			taskList = append(taskList, task)
		}
	}

	if acc != nil {
		log.Warningf(ctx, "Could not obtain some updated registry settings: %v", acc)
	}

	// Apply tasks for updated settings
	acc = nil
	for _, d := range db.GetAll() {
		acc = errors.Join(acc, d.SubmitDeferredTasks(taskList...))
	}

	if acc != nil {
		return fmt.Errorf("could not submit new token to certain distros: %v", acc)
	}

	return nil
}

// getNewSubscription checks if the subscription has changed since the last time it was called. If so, the new subscription
// is returned in the form of a task.
func getNewSubscription(ctx context.Context, cacheDir string, conf *config.Config, db *database.DistroDB) (task.Task, error) {
	proToken, _, err := conf.Subscription(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve current subscription: %v", err)
	}

	isNew, err := valueIsNew(filepath.Join(cacheDir, "subscription.csum"), []byte(proToken))
	if err != nil {
		log.Warningf(ctx, "could not update checksum for Ubuntu Pro subscription: %v", err)
	}
	if isNew {
		return nil, nil
	}

	log.Debug(ctx, "New Ubuntu Pro subscription settings detected in registry")
	return tasks.ProAttachment{Token: proToken}, nil
}

// valueIsNew detects if the current value is different from the last time it was used.
// The return value is usable even if error is returned.
func valueIsNew(cachePath string, newValue []byte) (new bool, err error) {
	var newCheckSum []byte
	if len(newValue) != 0 {
		tmp := sha512.Sum512(newValue)
		newCheckSum = tmp[:]
	}

	defer decorateUpdateCache(&new, &err, cachePath, newCheckSum)

	oldChecksum, err := os.ReadFile(cachePath)
	if errors.Is(err, fs.ErrNotExist) {
		// File not found: there was no value before
		oldChecksum = nil
	} else if err != nil {
		return true, fmt.Errorf("could not read old value: %v", err)
	}

	if slices.Equal(oldChecksum, newCheckSum) {
		return false, nil
	}

	return true, nil
}

// decorateUpdateCache acts depending on caller's return values (hence decorate).
// It stores the new checksum to the cachefile. Any errors are joined to *err.
func decorateUpdateCache(new *bool, err *error, cachePath string, newCheckSum []byte) {
	writeCacheErr := func() error {
		// If the value is empty, we remove the file.
		// This preserves this function's idempotency.
		if len(newCheckSum) == 0 {
			err := os.Remove(cachePath)
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			if err != nil {
				return fmt.Errorf("could not remove old checksum: %v", err)
			}
			return nil
		}

		// Value is unchanged: don't write to file
		if !*new {
			return nil
		}

		// Update to file
		if err := os.WriteFile(cachePath, newCheckSum[:], 0600); err != nil {
			return fmt.Errorf("could not write checksum to cache: %v", err)
		}

		return nil
	}()

	*err = errors.Join(*err, writeCacheErr)
}
