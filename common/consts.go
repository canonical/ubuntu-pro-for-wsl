// Package consts defines the constants used by the project
package common

const (
	// TEXTDOMAIN is the gettext domain for l10n.
	TEXTDOMAIN = `ubuntu-pro`

	// TODO: CacheRelativePath is the relative path name used in user's cache dir to store process transient data.
	// Windows: $HOME/{ChacheRelativePath}
	// WSL:     $WINHOME/{ChacheRelativePath}
	CacheRelativePath = "AppData/Local/Ubuntu Pro"

	// ListeningPortFileName corresponds to the base name of the file hosting the addressing of our GRPC server.
	ListeningPortFileName = "addr"
)
