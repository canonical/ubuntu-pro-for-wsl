package distro

import (
	"context"
	"errors"
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

	// This instance was created through
	// an install command from Landscape
	CreatedByLandscape bool
}

// isValid checks that the properties against the registry.
func (id identity) isValid() (ok bool) {
	distro := wsl.NewDistro(id.ctx, id.Name)

	inRegistry, err := distro.GUID()
	if errors.Is(err, wsl.ErrNotExist) {
		// Distro was not registered
		return false
	} else if err != nil {
		// Registry inaccessible
		panic(fmt.Errorf("could not access the Windows Registry: %v", err))
	}

	if id.GUID != inRegistry {
		// Distro was unregistered and re-registered
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
