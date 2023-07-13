package common

import (
	_ "embed"
)

// Version is the version number for Ubuntu Pro For Windows
//
//go:embed version
var Version string
