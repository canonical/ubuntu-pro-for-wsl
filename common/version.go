package common

import (
	_ "embed"
)

// Version is the version number for Ubuntu Pro for WSL
//
//go:embed version
var Version string
