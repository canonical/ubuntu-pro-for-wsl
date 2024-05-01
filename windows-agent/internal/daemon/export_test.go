package daemon

const (
	MockError                    = mockError
	EmptyList                    = emptyList
	NoHyperVAdapterInList        = noHyperVAdapterInList
	SingleHyperVAdapterInList    = singleHyperVAdapterInList
	MultipleHyperVAdaptersInList = multipleHyperVAdaptersInList
)

type MockIPAdaptersState = mockIPAdaptersState

var NewHostIPConfigMock = newHostIPConfigMock

func WithWslSystemCmd(cmd []string, cmdEnv []string) Option {
	return func(o *options) {
		o.wslSystemCmd = cmd
		o.wslCmdEnv = cmdEnv
	}
}

func WithGetAdaptersAddressesFunction(getAddr getAdaptersAddressesFunc) Option {
	return func(o *options) {
		o.getAdaptersAddresses = getAddr
	}
}
