package database

import (
	"context"
	"sync"

	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/distro"
)

type SerializableDistro = serializableDistro

// NewDistro is a wrapper around newDistro so as to make it accessible to tests.
func (in SerializableDistro) NewDistro(ctx context.Context, storageDir string, startupMu *sync.Mutex) (*distro.Distro, error) {
	return in.newDistro(ctx, storageDir, startupMu)
}

// NewSerializableDistro is a wrapper around newSerializableDistro so as to make it accessible to tests.
func NewSerializableDistro(d *distro.Distro) SerializableDistro {
	return newSerializableDistro(d)
}

// DistroNames returns the names of all distros in the database.
func (db *DistroDB) DistroNames() (out []string) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	for _, d := range db.distros {
		out = append(out, d.Name())
	}
	return out
}
