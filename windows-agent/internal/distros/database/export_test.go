package database

import "github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distros/distro"

type SerializableDistro = serializableDistro

// NewDistro is a wrapper around newDistro so as to make it accessible to tests.
func (in SerializableDistro) NewDistro(storageDir string) (*distro.Distro, error) {
	return in.newDistro(storageDir)
}

// NewSerializableDistro is a wrapper around newSerializableDistro so as to make it accessible to tests.
//
//nolint:revive // unexported-return false positive! SerializableDistro is exported, even if it is an alias to an unexported type.
func NewSerializableDistro(d *distro.Distro) SerializableDistro {
	return newSerializableDistro(d)
}

// DistroNames returns the names of all distros in the database.
func (db *DistroDB) DistroNames() (out []string) {
	for _, d := range db.distros {
		out = append(out, d.Name())
	}
	return out
}
