package distro

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	wsl "github.com/ubuntu/gowsl"
)

// identity contains persistent and uniquely identifying information about the distro.
type identity struct {
	Name string
	GUID uuid.UUID

	// This context contains GoWSL's backend
	ctx context.Context
}

// Properties contains persistent non-identifying information about the distro.
type Properties struct {
	// Release info
	DistroID   string
	VersionID  string
	PrettyName string

	// Instance info
	Hostname string

	// Ubuntu Pro
	ProAttached bool
}

// isValid checks that the properties against the registry.
// TODO: check all calls for isValid(), and if when !ok -> return error in the caller, just returns an error.
func (id identity) isValid() (ok bool) {
	distro := wsl.NewDistro(id.ctx, id.Name)

	// Ensuring distro still exists.
	registered, err := distro.IsRegistered()
	if err != nil {
		panic(fmt.Errorf("could not access the registry: %v", err))
	}
	if !registered {
		return false
	}

	// Ensuring it has not been unregistered and re-registered again.
	inProperties := id.GUID
	inRegistry, err := distro.GUID()
	if err != nil {
		panic(fmt.Errorf("could not access the registry: %v", err))
	}
	if inProperties != inRegistry {
		return false
	}

	// Distro with matching name and GUID exists
	return true
}

func (id identity) getDistro() (d wsl.Distro, err error) {
	if !id.isValid() {
		return d, &NotValidError{}
	}
	return wsl.NewDistro(id.ctx, id.Name), nil
}
