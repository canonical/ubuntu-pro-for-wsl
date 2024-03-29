package database

import (
	"context"
	"sync"

	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/distro"
	"github.com/google/uuid"
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
func (in serializableDistro) newDistro(ctx context.Context, storageDir string, startupMu *sync.Mutex) (*distro.Distro, error) {
	GUID, err := uuid.Parse(in.GUID)
	if err != nil {
		return nil, err
	}
	return distro.New(ctx, in.Name, in.Properties, storageDir, startupMu, distro.WithGUID(GUID))
}

// newSerializableDistro takes the information in distro.Distro relevant to the database
// and stores it the helper object.
func newSerializableDistro(d *distro.Distro) serializableDistro {
	return serializableDistro{
		Name:       d.Name(),
		GUID:       d.GUID(),
		Properties: d.Properties(),
	}
}
