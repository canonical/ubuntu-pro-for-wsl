//go:build gowslmock

package daemon

import "os"

func init() {
	m := newHostIPConfigMock(multipleHyperVAdaptersInList)

	defaultOptions = options{
		wslSystemCmd: []string{
			os.Args[0],
			"-test.run",
			"TestWithWslSystemMock",
			"--",
			"wslinfo",
			"--networking-mode",
			"-n",
			"nat",
		},
		wslCmdEnv:            []string{"GO_WANT_HELPER_PROCESS=1"},
		getAdaptersAddresses: m.GetAdaptersAddresses,
	}
}
