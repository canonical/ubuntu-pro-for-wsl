// Package consts defines the constants used by the project
package consts

import log "github.com/sirupsen/logrus"

const (
	// TEXTDOMAIN is the gettext domain for l10n.
	TEXTDOMAIN = `ubuntu-pro`

	// DefaultLogLevel is the default logging level selected without any option.
	DefaultLogLevel = log.WarnLevel

	// CacheBaseDirectory is the directory name used in user's cache dir to store process transient data.
	CacheBaseDirectory = "Ubuntu Pro"
)
