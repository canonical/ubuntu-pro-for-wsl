// Package distro abstracts a WSL distribution and deals manages all iteractions
// with it.
package distro

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"

	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/initialtasks"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/task"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/worker"
	log "github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-windows/wslserviceapi"
	"github.com/google/uuid"
	"github.com/ubuntu/decorate"
	"github.com/ubuntu/gowsl"
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
	Properties

	// invalidated is an internal value if distro can't be contacted through GRPC
	invalidated atomic.Bool

	ctx context.Context

	worker workerInterface
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
	initialTasks          *initialtasks.InitialTasks
	taskProcessingContext context.Context
	newWorkerFunc         func(context.Context, *Distro, string, *initialtasks.InitialTasks) (workerInterface, error)
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

// WithInitialTasks is an optional parameter for distro.New so that the
// distro con perform the tasks expected from any new distro.
func WithInitialTasks(i *initialtasks.InitialTasks) Option {
	return func(o *options) {
		o.initialTasks = i
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
		newWorkerFunc: func(ctx context.Context, d *Distro, dir string, init *initialtasks.InitialTasks) (workerInterface, error) {
			return worker.New(ctx, d, dir, worker.WithInitialTasks(init))
		},
	}

	for _, f := range args {
		f(&opts)
	}

	id := identity{
		Name: name,
		GUID: opts.guid,
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
		if !id.isValid(ctx) {
			return nil, fmt.Errorf("no distro with this name and GUID %q in registry: %w", id.GUID.String(), &NotValidError{})
		}
	}

	distro = &Distro{
		identity:   id,
		Properties: props,
	}

	if err := os.MkdirAll(storageDir, 0600); err != nil {
		return nil, err
	}

	distro.worker, err = opts.newWorkerFunc(opts.taskProcessingContext, distro, storageDir, opts.initialTasks)
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
}

// Invalidate sets the invalid flag to true. The state of this flag can be read with IsValid.
// This is irreversible, once the flag is true there is no way of setting it bag to false.
func (d *Distro) Invalidate(err error) {
	if err == nil {
		log.Warningf(context.TODO(), "distro %q: attempted to invalidate with a nil error", d.Name())
		return
	}

	updated := d.invalidated.CompareAndSwap(false, true)
	if updated {
		log.Debugf(context.TODO(), "distro %q: marked as no longer valid", d.Name())
	}
}

// IsValid checks the registry to see if the distro is valid. If it is not, an internal flag
// is set and all subsequent calls will return false automatically. This flag may also be set
// directly via Invalidate.
func (d *Distro) IsValid() bool {
	if d.invalidated.Load() {
		return false
	}

	if !d.identity.isValid(d.ctx) {
		d.Invalidate(&NotValidError{})
		return false
	}

	return true
}

// KeepAwake ensures the distro is started by running a long life command inside
// WSL. It will thus start it if it's not already the case.
// Cancelling the context will remove this keep awake lock, but does not necessarily mean
// that the distribution will be shutdown right away.
//
// The command is reentrant, and you need to cancel the amount of time you keep it awake.
func (d *Distro) KeepAwake(ctx context.Context) error {
	if !d.IsValid() {
		return &NotValidError{}
	}

	wslDistro := gowsl.NewDistro(ctx, d.identity.Name)

	cmd := wslDistro.Command(ctx, "sleep infinity")
	err := cmd.Start()
	if err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		_ = cmd.Wait()
	}()

	return nil
}
