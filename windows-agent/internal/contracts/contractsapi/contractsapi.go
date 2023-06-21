// Package contractsapi exports some constants defining the Contracts Server backend REST API
package contractsapi

const (
	// Version is the current Contracts Server REST API version.
	Version = "/v1"

	//TokenPath is the path where clients should GET the Azure AD ephemeral token.
	TokenPath = "/token"
	// SubscriptionPath is the path where clients should POST the user JWT to notify the CS backend of changes in the current user subscription.
	SubscriptionPath = "/subscription"

	// TokenMaxSize is a safe token response size - tests with the real MS APIs suggested that those tokens will stay in between 1.2kB to 1.7kB.
	// Our Pro Token is much, much smaller.
	TokenMaxSize = 4096

	//nolint:gosec // G101 false positive, this is not a credential
	// ADTokenKey is the JSON key of the response payload of the /token endpoint.
	ADTokenKey = "azure_ad_token"
	// JWTKey is the JSON key of the request payload of the /susbcription endpoint.
	JWTKey = "ms_store_id_key"
	// ProTokenKey is the JSON key of a successful response payload of the /subscription endpoint.
	ProTokenKey = "contract_token"
)
