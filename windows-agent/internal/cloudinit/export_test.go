package cloudinit

import (
	"os/user"
	"testing"
)

// InjectUser is a function to inject a user getter function into the CloudInit object.
func (c *CloudInit) InjectUser(f func() (*user.User, error)) {
	if !testing.Testing() {
		panic("InjectUser can only be used in tests")
	}

	c.user = f
}
