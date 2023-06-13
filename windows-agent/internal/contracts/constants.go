package contracts

const (
	apiVersion = "/v1"

	// endpoints.
	tokenPath        = "/token"
	subscriptionPath = "/subscription"

	// A safe token response size - tests with the real MS APIs suggested that those tokens will stay in between 1.2kB to 1.7kB.
	// Our Pro Token is much, much smaller.
	apiTokenMaxSize = 4096

	// JSON keys commonly referred in the Contracts Server backend REST API.
	//nolint:gosec // G101 false positive, this is not a credential
	adTokenKey  = "azure_ad_token"
	jwtKey      = "ms_store_id_key"
	proTokenKey = "contract_token"
)
