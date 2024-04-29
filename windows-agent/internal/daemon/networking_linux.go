package daemon

import (
	"context"
	"os/exec"
)

type ipConfig struct {
	m mockIPConfig
}

func newIPConfig() ipConfig {
	return ipConfig{
		m: newHostIPConfigMock(ok),
	}
}

type wslSystem struct {
	m *mockWslSystem
}

func newWslSystem() wslSystem {
	return wslSystem{
		m: newWslSystemMock("nat", []string{
			"UP4W_MOCK_EXECUTABLE=1",
			"UP4W_MOCK_EXECUTABLE=1",
		}, false),
	}
}

func (i ipConfig) getAdaptersAddresses() (head ipAdapterAddresses, err error) {
	return i.m.getAdaptersAddresses()
}

func (wsl wslSystem) Command(ctx context.Context, name string, arg ...string) *exec.Cmd {
	wsl.m.netmode = "nat"
	return wsl.m.Command(ctx, name, arg...)
}
