// Package apidef exports some constants defining the Contracts Server backend REST API
package apidef

const (
	Version = "/v1"

	// endpoints.
	TokenPath        = "/token"
	SubscriptionPath = "/subscription"

	// A safe token response size - tests with the real MS APIs suggested that those tokens will stay in between 1.2kB to 1.7kB.
	// Our Pro Token is much, much smaller.
	TokenMaxSize = 4096

	// JSON keys commonly referred in the Contracts Server backend REST API.
	//nolint:gosec // G101 false positive, this is not a credential
	ADTokenKey  = "azure_ad_token"
	JWTKey      = "ms_store_id_key"
	ProTokenKey = "contract_token"
)
