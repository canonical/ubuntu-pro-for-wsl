package distro

import (
	"github.com/ubuntu/decorate"
	"golang.org/x/sys/windows"

	wsl "github.com/ubuntu/gowsl"
)

// identity contains persistent and uniquely identifying information about the distro.
type identity struct {
	Name string
	GUID windows.GUID
}

// Properties contains persistent non-identifying information about the distro.
type Properties struct {
	// Release info
	DistroID   string
	VersionID  string
	PrettyName string

	// Ubuntu Pro
	ProAttached bool
}

// IsValid checks that the properties against the registry.
// TODO: check all calls for IsValid(), and if when !ok -> return error in the caller, just returns an error.
func (id identity) IsValid() (ok bool, err error) {
	decorate.OnError(&err, "combination does not match the registry: {distroName: %q, GUID: %q}", id.Name, id.GUID)

	distro := wsl.NewDistro(id.Name)

	// Ensuring distro still exists.
	registered, err := distro.IsRegistered()
	if err != nil {
		return false, err
	}
	if !registered {
		return false, nil
	}

	// Ensuring it has not been unregistered and re-registered again.
	inProperties := id.GUID
	inRegistry, err := distro.GUID()
	if err != nil {
		return false, err
	}
	if inProperties != inRegistry {
		return false, nil
	}

	// Distro with matching name and GUID exists
	return true, nil
}
