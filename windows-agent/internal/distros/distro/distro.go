// Package distro abstracts a WSL distribution and deals manages all iteractions
// with it.
package distro

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"

	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/task"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/worker"
	log "github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-windows/wslserviceapi"
	"github.com/google/uuid"
	"github.com/ubuntu/decorate"
	wsl "github.com/ubuntu/gowsl"
	"google.golang.org/grpc"
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
	Client() wslserviceapi.WSLClient
	SetConnection(*grpc.ClientConn)
	SubmitTasks(...task.Task) error
	Stop(context.Context)
}

// NotValidError is a type returned when the (distroName, GUID) combination is not in the registry.
type NotValidError struct{}

func (*NotValidError) Error() string {
	return "distro does not exist"
}

type options struct {
	guid                  uuid.UUID
	provisioning          worker.Provisioning
	taskProcessingContext context.Context
	newWorkerFunc         func(context.Context, *Distro, string, worker.Provisioning) (workerInterface, error)
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

// WithProvisioning allows for providing a worker.Provisioning. If that is done,
// it'll be queried for the provisioning tasks and these will be submitted.
func WithProvisioning(c worker.Provisioning) Option {
	return func(o *options) {
		o.provisioning = c
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
func New(ctx context.Context, name string, props Properties, storageDir string, args ...Option) (distro *Distro, err error) {
	decorate.OnError(&err, "could not initialize distro %q", name)

	var nilGUID uuid.UUID
	opts := options{
		guid:                  nilGUID,
		taskProcessingContext: context.Background(),
		newWorkerFunc: func(ctx context.Context, d *Distro, dir string, provisioning worker.Provisioning) (workerInterface, error) {
			return worker.New(ctx, d, dir, worker.WithProvisioning(provisioning))
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

	distro = &Distro{
		identity:   id,
		properties: props,
		stateManager: &stateManager{
			distroIdentity: id,
		},
	}

	if err := os.MkdirAll(storageDir, 0600); err != nil {
		return nil, err
	}

	distro.worker, err = opts.newWorkerFunc(opts.taskProcessingContext, distro, storageDir, opts.provisioning)
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

// Client returns the client to the WSL task service.
// Client returns nil when no connection is set up.
func (d *Distro) Client() (wslserviceapi.WSLClient, error) {
	if !d.IsValid() {
		return nil, &NotValidError{}
	}
	return d.worker.Client(), nil
}

// SetConnection removes the connection associated with the distro.
func (d *Distro) SetConnection(conn *grpc.ClientConn) error {
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

	if !d.identity.isValid() {
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
