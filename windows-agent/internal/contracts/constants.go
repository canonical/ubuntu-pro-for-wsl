package contracts

const (
	apiVersion = "/v1"

	// endpoints
	getToken         = "/token"
	postSubscription = "/subscription"

	// a safe token response size - tests with the real MS APIs suggested that those tokens will stay in between 1.2kB to 1.7kB.
	// Our Pro Token is much, much smaller.
	apiTokenMaxSize = 4096

	// JSON keys commonly referred in the Contracts Server backend REST API
	jsonKeyAdToken  = "azure_ad_token"
	jsonKeyJwt      = "ms_store_id_key"
	jsonKeyProToken = "contract_token"
)
