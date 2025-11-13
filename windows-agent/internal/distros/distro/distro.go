// Package distro abstracts a WSL distribution and deals manages all iteractions
// with it.
package distro

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	log "github.com/canonical/ubuntu-pro-for-wsl/common/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/task"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/worker"
	"github.com/google/uuid"
	"github.com/ubuntu/decorate"
	wsl "github.com/ubuntu/gowsl"
)

// Distro is a wrapper around gowsl.Distro that tracks both the distroname and
// the GUID, ensuring that the distro has not been unregistered and re-registered.
type Distro struct {
	// Identity contains non-volatile information that is stored in the database
	// and is used to uniquely identify distros
	identity

	// Properties contains non-volatile information that is stored in the database
	properties   Properties
	propertiesMu sync.RWMutex

	// invalidated is an internal value if distro can't be contacted through GRPC
	invalidated atomic.Bool

	worker       workerInterface
	stateManager *stateManager
}

// workerInterface is an interface that is implements the task processing worker. It is intended
// for woker.workerInterface, and to allow dependency injection in tests.
type workerInterface interface {
	IsActive() bool
	Connection() worker.Connection
	SetConnection(worker.Connection)
	SubmitTasks(...task.Task) error
	SubmitDeferredTasks(...task.Task) error
	EnqueueDeferredTasks()
	Stop(context.Context)
}

// NotValidError is a type returned when the (distroName, GUID) combination is not in the registry.
type NotValidError struct{}

func (*NotValidError) Error() string {
	return "distro does not exist"
}

type options struct {
	guid                  uuid.UUID
	taskProcessingContext context.Context
	newWorkerFunc         func(context.Context, *Distro, string) (workerInterface, error)
}

// Option is an optional argument for distro.New.
type Option func(*options)

// WithGUID is an optional parameter for distro.New that enforces GUID
// validation.
func WithGUID(guid uuid.UUID) Option {
	return func(o *options) {
		o.guid = guid
	}
}

// New creates a new Distro object after searching for a distro with the given name.
//
//   - If identity.Name is not registered, a DistroDoesNotExist error is returned.
//
//   - Otheriwse, identity.GUID will be validated against the registry. In case of mismatch,
//     a DistroDoesNotExist error is returned
//
//   - To avoid the latter check, you can pass a default-constructed identity.GUID. In that
//     case, the distro will be created with its currently registered GUID.
func New(ctx context.Context, name string, props Properties, storageDir string, startupMu *sync.Mutex, args ...Option) (distro *Distro, err error) {
	decorate.OnError(&err, "could not initialize distro %q", name)

	var nilGUID uuid.UUID
	opts := options{
		guid:                  nilGUID,
		taskProcessingContext: context.Background(),
		newWorkerFunc: func(ctx context.Context, d *Distro, dir string) (workerInterface, error) {
			return worker.New(ctx, d, dir)
		},
	}

	for _, f := range args {
		f(&opts)
	}

	id := identity{
		Name: name,
		GUID: opts.guid,
		ctx:  ctx,
	}

	// GUID is not initialized.
	if id.GUID == nilGUID {
		d := wsl.NewDistro(ctx, name)
		guid, err := d.GUID()
		if err == nil {
			id.GUID = guid
		} else {
			return nil, fmt.Errorf("no distro with this name exists: %w", &NotValidError{})
		}
	} else {
		// Check the name/GUID pair is valid.
		if !id.isValid() {
			return nil, fmt.Errorf("no distro with this name and GUID %q in registry: %w", id.GUID.String(), &NotValidError{})
		}
	}

	if startupMu == nil {
		return nil, errors.New("startup mutex must not be nil")
	}

	distro = &Distro{
		identity:   id,
		properties: props,
		stateManager: &stateManager{
			distroIdentity: id,
			startupMu:      startupMu,
		},
	}

	distro.worker, err = opts.newWorkerFunc(opts.taskProcessingContext, distro, storageDir)
	if err != nil {
		return nil, err
	}

	return distro, nil
}

func (d *Distro) String() string {
	return fmt.Sprintf("Distro{ name: %q, guid: %q }", d.Name(), d.GUID())
}

// Name is a getter for the distro's name.
func (d *Distro) Name() string {
	return d.identity.Name
}

// GUID is a getter for the distro's GUID.
func (d *Distro) GUID() string {
	return d.identity.GUID.String()
}

// Properties is a getter for the distro's Properties.
func (d *Distro) Properties() Properties {
	d.propertiesMu.RLock()
	defer d.propertiesMu.RUnlock()

	return d.properties
}

// SetProperties sets the specified properties, and returns true if the set properties are
// different from the original ones.
func (d *Distro) SetProperties(p Properties) bool {
	d.propertiesMu.Lock()
	defer d.propertiesMu.Unlock()

	if d.properties == p {
		return false
	}
	d.properties = p
	return true
}

// IsActive returns true when the distro is running, and there exists an active
// connection to its GRPC service.
func (d *Distro) IsActive() (bool, error) {
	if !d.IsValid() {
		return false, &NotValidError{}
	}
	return d.worker.IsActive(), nil
}

// Connection returns the Connection to the WSL task service.
// Connection returns nil when no connection is set up.
func (d *Distro) Connection() (worker.Connection, error) {
	if !d.IsValid() {
		return nil, &NotValidError{}
	}
	return d.worker.Connection(), nil
}

// SetConnection removes the connection associated with the distro.
func (d *Distro) SetConnection(conn worker.Connection) error {
	// Allowing IsValid check to be bypassed when resetting the connection
	if conn == nil {
		d.worker.SetConnection(nil)
		return nil
	}

	if !d.IsValid() {
		return &NotValidError{}
	}
	d.worker.SetConnection(conn)
	return nil
}

// SubmitTasks enqueues one or more task on our current worker list.
// See Worker.SubmitTasks for details.
func (d *Distro) SubmitTasks(tasks ...task.Task) (err error) {
	if !d.IsValid() {
		return &NotValidError{}
	}
	return d.worker.SubmitTasks(tasks...)
}

// SubmitDeferredTasks enqueues one or more task on our current worker list.
// See Worker.SubmitDeferredTasks for details.
func (d *Distro) SubmitDeferredTasks(tasks ...task.Task) (err error) {
	if !d.IsValid() {
		return &NotValidError{}
	}
	return d.worker.SubmitDeferredTasks(tasks...)
}

// EnqueueDeferredTasks takes all deferred tasks and promotes them
// to regular tasks.
func (d *Distro) EnqueueDeferredTasks() {
	d.worker.EnqueueDeferredTasks()
}

// Cleanup releases all resources associated with the distro.
func (d *Distro) Cleanup(ctx context.Context) {
	if d == nil {
		return
	}
	d.worker.Stop(ctx)
	d.stateManager.reset()
}

// Invalidate sets the invalid flag to true. The state of this flag can be read with IsValid.
// This is irreversible, once the flag is true there is no way of setting it bag to false.
func (d *Distro) Invalidate(ctx context.Context) {
	updated := d.invalidated.CompareAndSwap(false, true)
	if updated {
		log.Infof(ctx, "distro %q: marked as no longer valid", d.Name())
	}
}

// IsValid checks the registry to see if the distro is valid. If it is not, an internal flag
// is set and all subsequent calls will return false automatically. This flag may also be set
// directly via Invalidate.
func (d *Distro) IsValid() bool {
	if d.invalidated.Load() {
		return false
	}

	if !d.isValid() {
		d.Invalidate(d.ctx)
		return false
	}

	return true
}

// State returns the state of the WSL distro, as implemeted by GoWSL.
func (d *Distro) State() (s wsl.State, err error) {
	return d.stateManager.state()
}

// LockAwake ensures that the distro will stay awake until ReleaseAwake is called.
// ReleaseAwake must be called the same amount of times for the distro to be
// allowed to stop.
//
// The distro is guaranteed to be running by the time this function returns,
// otherwise an error is returned.
func (d *Distro) LockAwake() error {
	if !d.IsValid() {
		return &NotValidError{}
	}
	return d.stateManager.lock(d.ctx)
}

// ReleaseAwake undoes the last call to LockAwake. If this was the last call, the
// distro is allowed to auto-shutdown.
func (d *Distro) ReleaseAwake() error {
	if !d.IsValid() {
		return &NotValidError{}
	}
	return d.stateManager.release()
}

// Uninstall unregisters the distro and uninstalls its associated Appx.
func (d *Distro) Uninstall(ctx context.Context) error {
	distro, err := d.getDistro()
	if err != nil {
		return err
	}

	return distro.Uninstall(ctx)
}
