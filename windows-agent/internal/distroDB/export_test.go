package distroDB

import "github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/distro"

type SerializableDistro = serializableDistro

// NewDistro is a wrapper around newDistro so as to make it accessible to tests.
func (in SerializableDistro) NewDistro() (*distro.Distro, error) {
	return in.newDistro()
}

// NewSerializableDistro is a wrapper around newSerializableDistro so as to make it accessible to tests.
//
//nolint: revive
// unexported-return false positive! SerializableDistro is exported, even if it is an alias to an unexported type.
func NewSerializableDistro(d *distro.Distro) SerializableDistro {
	return newSerializableDistro(d)
}
