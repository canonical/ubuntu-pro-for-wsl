// Package common defines the constants used by the project
package common

const (
	// TEXTDOMAIN is the gettext domain for l10n.
	TEXTDOMAIN = `ubuntu-pro`

	// LocalAppDataDir is the relative path name used in user's cache dir to store process transient data.
	//  ${env:LocalAppData}/{LocalAppDataDir}
	LocalAppDataDir = "Ubuntu Pro"

	// ListeningPortFileName corresponds to the base name of the file hosting the addressing of our GRPC server.
	ListeningPortFileName = ".ubuntupro"

	// MsStoreProductID is the ID of the product in the Microsoft Store
	//
	// TODO: Replace with real product ID.
	MsStoreProductID = "9P25B50XMKXT"
)
