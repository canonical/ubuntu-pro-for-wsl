package netmonitoring

// DefaultAPIProvider on Linux must delegate to the mock implementation.
func DefaultAPIProvider() (DevicesAPI, error) {
	panic("defaultNetAdaptersAPIProvider is not implemented on Linux without a mock")
}
