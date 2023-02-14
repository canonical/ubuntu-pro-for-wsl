package distro

import (
	"context"
	"fmt"
	"strings"
	"sync"

	log "github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/grpc/logstreamer"
	"github.com/ubuntu/decorate"
	wsl "github.com/ubuntu/gowsl"
	"golang.org/x/sys/windows"
	"google.golang.org/grpc"
)

const taskQueueBufferSize = 100

// Distro is a wrapper around gowsl.Distro that tracks both the distroname and
// the GUID, ensuring that the distro has not been unregistered and re-registered.
type Distro struct {
	// Identity contains non-volatile information that is stored in the database
	// and is used to uniquely identify distros
	identity

	// Properties contains non-volatile information that is stored in the database
	Properties

	// UnreachableErr is not nil if distro can't be contacted through GRPC
	UnreachableErr error

	// The following fields may change without afecting long-term storage of the distro
	cancel          context.CancelFunc
	tasks           chan Task
	tasksInProgress chan struct{}

	conn   *grpc.ClientConn
	connMu *sync.RWMutex
}

// NotExistError is a type returned when the (distroName, GUID) combination is not in the registry.
type NotExistError struct{}

func (*NotExistError) Error() string {
	return "distro does not exist"
}

type options struct {
	guid windows.GUID
}

// Option is an optional argument for distro.New.
type Option func(*options)

// WithGUID is an optional parameter for distro.New that enforces GUID
// validation.
func WithGUID(guid windows.GUID) Option {
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
func New(name string, props Properties, args ...Option) (distro *Distro, err error) {
	decorate.OnError(&err, "could not initialize distro %q", name)

	var opts options
	for _, f := range args {
		f(&opts)
	}

	id := identity{
		Name: name,
		GUID: opts.guid,
	}

	// GUID is not initialized.
	var nilGUID windows.GUID
	if id.GUID == nilGUID {
		d := wsl.NewDistro(name)
		guid, err := d.GUID()
		if err == nil {
			id.GUID = guid
		} else {
			return nil, fmt.Errorf("no distro with this name exists: %w", &NotExistError{})
		}
	} else {
		// Check the name/GUID pair is valid.
		valid, err := id.IsValid()
		if err != nil {
			return nil, err
		}
		if !valid {
			return nil, fmt.Errorf("no distro with this name and GUID %q in registry: %w", id.GUID.String(), &NotExistError{})
		}
	}

	distro = &Distro{
		identity:   id,
		Properties: props,

		tasks:  make(chan Task, taskQueueBufferSize),
		connMu: &sync.RWMutex{},
	}

	distro.startProcessingTasks(context.TODO())

	return distro, nil
}

func (d Distro) String() string {
	return fmt.Sprintf("Distro{ name: %q, guid: %q }", d.Name, strings.ToLower(d.GUID.String()))
}

// Cleanup releases all resources associated with the distro.
func (d *Distro) Cleanup(ctx context.Context) {
	err := d.stopProcessingTasks(ctx)
	if err != nil {
		log.Infof(ctx, "Distro %q: error during cleanup: %v", d.Name, err)
	}
}

// getWSLDistro gets underlying GoWSL distro after verifying it.
func (d Distro) getWSLDistro() (wsl.Distro, error) {
	verified, err := d.IsValid()
	if err != nil {
		return wsl.NewDistro(""), err
	}
	if !verified {
		return wsl.NewDistro(""), fmt.Errorf("distro with name %q and GUID %q not found in registry: %w", d.Name, d.GUID.String(), &NotExistError{})
	}
	return wsl.NewDistro(d.Name), nil
}

// keepAwake ensures the distro is started by running a long life command inside
// WSL. It will thus start it if it's not already the case.
// Cancelling the context will remove this keep awake lock, but does not necessarily mean
// that the distribution will be shutdown right away.
//
// The command is reentrant, and you need to cancel the amount of time you keep it awake.
func (d *Distro) keepAwake(ctx context.Context) error {
	wslDistro, err := d.getWSLDistro()
	if err != nil {
		return err
	}

	cmd := wslDistro.Command(ctx, "sleep infinity")
	err = cmd.Start()
	if err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		_ = cmd.Wait()
	}()

	return nil
}
