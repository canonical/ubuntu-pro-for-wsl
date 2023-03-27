package common

import (
	_ "embed"
)

// Version contains the version of Ubuntu-Pro-for-Windows. This string is the same for all components.
//
//go:embed version
var Version string
