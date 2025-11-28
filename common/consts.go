// Package common defines the constants used by the project
package common

const (
	// TEXTDOMAIN is the gettext domain for l10n.
	TEXTDOMAIN = `ubuntu-pro`

	// LocalAppDataDir is the relative path name used to store data private to the Appx.
	//  ${env:LocalAppData}/{LocalAppDataDir}
	LocalAppDataDir = "Ubuntu Pro"

	// UserProfileDir is the relative path name used to store data that needs to be shared between components.
	//  ${env:UserProfile}/{UserProfileDir}
	UserProfileDir = ".ubuntupro"

	// ListeningPortFileName corresponds to the base name of the file hosting the addressing of our GRPC server.
	ListeningPortFileName = ".address"

	// MsStoreProductID is the ID of the product in the Microsoft Store.
	MsStoreProductID = "9PBDP6SFLM8G"

	// CertificatesDir is the agent's public subdirectory where the certificates are stored.
	CertificatesDir = "certs"

	// GRPCServerNameOverride is the name to override the server name in when configuring TLS for local clients.
	GRPCServerNameOverride = "UP4W"

	// RootCACertFileName is the name of the certificate file that identifies the root certificate authority in the PEM format.
	RootCACertFileName = "ca_cert.pem"

	// AgentCertFilePrefix is the file name prefix to identify the certificate/key pair of the agent in the PEM format.
	AgentCertFilePrefix = "agent"

	// ClientsCertFilePrefix is the file name prefix to identify the certificate/key pair of the clients (GUI and all WSL instances) in the PEM format.
	ClientsCertFilePrefix = "client"

	// CertificateSuffix is the file name suffix to the (public) certificate in the PEM format.
	CertificateSuffix = "_cert.pem"

	// KeySuffix is the file name suffix to the private key in the PEM format.
	KeySuffix = "_key.pem"
)
