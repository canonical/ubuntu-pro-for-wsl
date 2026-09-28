package daemon

// NewOpenat2RootForTest constructs an openat2Root with a specified fd for testing edge cases.
func NewOpenat2RootForTest(fd int) RootFs {
	return &openat2Root{fd: fd}
}

// ExpectedOwnerForTest returns the configured expected UID and GID for validation testing.
func ExpectedOwnerForTest() (uid, gid uint32) {
	return expectedUID, expectedGID
}
