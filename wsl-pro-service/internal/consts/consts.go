// Package consts defines the constants used by the project
package consts

import (
	log "github.com/sirupsen/logrus"
)

const (
	// DefaultLogLevel is the default logging level selected without any option.
	DefaultLogLevel = log.WarnLevel
)

// Version is the version of the service
//
// It is set at build time using the -ldflags option.
var Version = "Dev"
