// Package ubuntupro provides functions to manage the Ubuntu Pro subscription.
package ubuntupro

import (
	"context"
	"errors"
	"fmt"

	"github.com/canonical/ubuntu-pro-for-wsl/common"
	log "github.com/canonical/ubuntu-pro-for-wsl/common/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/config"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/database"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/tasks"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/ubuntupro/contracts"
	"github.com/ubuntu/decorate"
)

// Distribute sends the current subscription token to all distros.
func Distribute(ctx context.Context, db *database.DistroDB, ubuntuProToken string) {
	task := tasks.ProAttachment{
		Token: ubuntuProToken,
	}

	var err error
	for _, distro := range db.GetAll() {
		err = errors.Join(err, distro.SubmitTasks(task))
	}

	if err != nil {
		log.Warningf(ctx, "could not submit tasks to all distros: %v", err)
	}
}

// Config is a configuration manager for the Windows Agent.
type Config interface {
	Subscription() (string, config.Source, error)
	SetStoreSubscription(context.Context, string) error
}

// FetchFromMicrosoftStore contacts Ubuntu Pro's contract server and the Microsoft Store
// to check if the user has an active subscription that provides a pro token. If so, that token is used.
func FetchFromMicrosoftStore(ctx context.Context, conf Config, db *database.DistroDB, args ...contracts.Option) (err error) {
	defer decorate.OnError(&err, "config: could not validate subscription against Microsoft Store")

	_, src, err := conf.Subscription()
	if err != nil {
		return fmt.Errorf("could not get current subscription status: %v", err)
	}

	switch src {
	case config.SourceRegistry: // assumed to be assigned by the organization, so let's skip checking with MS Store and contracts backend.
		log.Debug(ctx, "Config: Skip checking with Microsoft Store: Organisation wide subscription is active")
		return nil
	case config.SourceMicrosoftStore:
		// Shortcut to avoid spamming the contract server
		// We don't need to request a new token if we have a non-expired one
		valid, err := contracts.ValidSubscription(args...)
		if err != nil {
			return fmt.Errorf("could not obtain current subscription status: %v", err)
		}

		if valid {
			log.Debug(ctx, "Config: Microsoft Store subscription is active")
			return nil
		}

		log.Debug(ctx, "Config: no valid Microsoft Store subscription")
	default:
	}

	log.Debug(ctx, "Config: attempting to obtain Ubuntu Pro token from the Microsoft Store")

	proToken, err := contracts.NewProToken(ctx, args...)
	if err != nil {
		err = fmt.Errorf("could not get the Ubuntu Pro token from the Microsoft Store: %v", err)
		log.Debugf(ctx, "Config: %v", err)
		return err
	}

	if proToken != "" {
		log.Debugf(ctx, "Config: obtained an Ubuntu Pro token from the Microsoft Store: %q", common.Obfuscate(proToken))
	}

	if err := conf.SetStoreSubscription(ctx, proToken); err != nil {
		return err
	}

	return nil
}
