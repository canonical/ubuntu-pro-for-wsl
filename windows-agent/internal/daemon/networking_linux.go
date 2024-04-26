package daemon

import (
	"context"
	"os/exec"
)

type ipConfig struct{}
type wslSystem struct{}

func (i ipConfig) getAdaptersAddresses() (head ipAdapterAddresses, err error) {
	panic("not implemented")
}

func (wsl wslSystem) Command(ctx context.Context, name string, arg ...string) *exec.Cmd {
	panic("not implemented")
}
