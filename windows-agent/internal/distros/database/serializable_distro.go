package database

import (
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/distro"
	"golang.org/x/sys/windows"
)

// serializableDistro is an helper struct for marshalling and unmarshalling into and
// from the database's disk backing.
//
// It contains all the persistent information in plain data structures,
// with none of the short-term information or functionality.
type serializableDistro struct {
	Name string
	GUID string
	distro.Properties
}

// newDistro calls distro.New with the name, GUID and properties specified
// in its inert counterpart.
func (in serializableDistro) newDistro(storageDir string) (*distro.Distro, error) {
	GUID, err := windows.GUIDFromString(in.GUID)
	if err != nil {
		return nil, err
	}
	return distro.New(in.Name, in.Properties, storageDir, distro.WithGUID(GUID))
}

// newSerializableDistro takes the information in distro.Distro relevant to the database
// and stores it the helper object.
func newSerializableDistro(d *distro.Distro) serializableDistro {
	return serializableDistro{
		Name:       d.Name,
		GUID:       d.GUID.String(),
		Properties: d.Properties,
	}
}
