package daemon

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	log "github.com/canonical/ubuntu-pro-for-wsl/common/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/daemon/netmonitoring"
	"github.com/google/uuid"
)

// NewAdapterCallback is called when new network adapters are added on the host.
// It must return true to continue receiving notifications or false to stop the subscription.
type NewAdapterCallback func(adapterNames []string) bool

// NetWatcher represents a subscription to events of network adapters added on the host.
type NetWatcher struct {
	ctx      context.Context
	cancel   context.CancelFunc
	callback NewAdapterCallback
	api      netmonitoring.DevicesAPI

	cache []string

	// err is a channel through which we join the current waiting goroutine.
	err chan error
}

// subscribe subscribes to the addition of network adapters on the host, calling the provided callback
// with a slice of new adapter names discovered by the time the OS triggers the notification.
func subscribe(ctx context.Context, callback NewAdapterCallback, opts *options) (*NetWatcher, error) {
	api, err := opts.netMonitoringProvider()
	if err != nil {
		return nil, fmt.Errorf("could not initialize the network adapter API: %v", err)
	}

	current, err := listAdapters(api)
	if err != nil {
		return nil, fmt.Errorf("could not get the current list of network adapters: %v", err)
	}

	nctx, cancel := context.WithCancel(ctx)
	// Ensures that the network adapter repository is closed when the context is cancelled so we don't need to do it explicitly.
	context.AfterFunc(nctx, api.Close)
	n := &NetWatcher{
		api:      api,
		ctx:      nctx,
		cancel:   cancel,
		callback: callback,
		err:      make(chan error, 1),
		cache:    current,
	}

	go func() {
		defer close(n.err)

		err := n.start()
		n.err <- err
		log.Debugf(context.Background(), "stopped monitoring network adapters: %v", err)
	}()
	return n, nil
}

// Stop blocks the caller until the subscription to the addition of network adapters on the host is stopped.
func (n *NetWatcher) Stop() error {
	n.cancel()

	// joins the goroutine that is waiting for network adapter changes.
	return <-n.err
}

// notify notifies the subscriber of the new network adapters added on the host.
// It returns true if the subscription should continue.
func (n *NetWatcher) notify() bool {
	// reloads the list of network adapters and their connection names from the registry
	current, err := listAdapters(n.api)
	if err != nil {
		log.Errorf(n.ctx, "could not get the current list of network adapters: %v", err)
		return true
	}
	// detects which network adapter was added, i.e. are in the current list but not in the cached list.
	added := difference(current, n.cache)
	if len(added) == 0 {
		return true
	}
	// updates the cache with the current list of network adapters.
	n.cache = current

	// finally calls the subscriber with the names of the new network adapters.
	return n.callback(added)
}

// start blocks a new goroutine on system notifications about network adapters on the host and notifies the subscriber,
// while ensuring that this object's context cancellation is respected.
func (n *NetWatcher) start() error {
	// Intentionally not closed to prevent potential panics due sending to a closed channel.
	waitCh := make(chan error)

	for {
		go func() {
			if err := n.api.WaitForDeviceChanges(); err != nil {
				waitCh <- fmt.Errorf("could not wait for network devices changes: %v", err)
				return
			}
			waitCh <- nil
		}()

		select {
		case <-n.ctx.Done():
			return n.ctx.Err()
		case err := <-waitCh:
			if err != nil {
				return err
			}
			if !n.notify() {
				return nil
			}
		}
	}
}

// Provides the current list of network adapters by their connection names as seen in the output of commands such as `ipconfig /all`.
func listAdapters(api netmonitoring.DevicesAPI) ([]string, error) {
	guids, err := api.ListDevices()
	if err != nil {
		return nil, fmt.Errorf("could not list network adapter GUIDs: %v", err)
	}

	// Filter out the entries that are not valid UUIDs.
	// When using the registry, there is at least one additional subkey named "Descriptions", which is not useful for this purpose.
	adapterGuids := filter(guids, func(guid string) bool {
		_, err := uuid.Parse(guid)
		return err == nil
	})

	adapterNames := make([]string, 0, len(adapterGuids))
	for _, guid := range adapterGuids {
		// Retrieves the connection name of the network adapter with the given GUID, which matches the device's Friendly Name.
		name, err := api.GetDeviceConnectionName(guid)
		if err != nil {
			return nil, err
		}
		adapterNames = append(adapterNames, name)
	}

	slices.Sort(adapterNames)
	return adapterNames, nil
}

// Given two sorted slices of strings, returns the elements that are in the first slice but not in the second.
func difference(a, b []string) []string {
	l := len(b)
	if l == 0 {
		return a
	}

	diff := make([]string, 0)
	for _, v := range a {
		pos, found := sort.Find(l, func(i int) int {
			return strings.Compare(v, b[i])
		})
		if !found || pos == l {
			diff = append(diff, v)
		}
	}
	return diff
}

// Given a slice of strings, returns a new slice containing only the elements for which the predicate returns true.
func filter(s []string, predicate func(string) bool) []string {
	res := make([]string, 0, len(s))
	for _, v := range s {
		if predicate(v) {
			res = append(res, v)
		}
	}
	return res
}
