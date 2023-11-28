// Package contractsapi exports some constants defining the Contracts Server backend REST API
package contractsapi

const (
	// Version is the current Contracts Server REST API version.
	Version = "/v1"

	// TokenPath is the path where clients should GET the Azure AD ephemeral token.
	TokenPath = "/token"
	// SubscriptionPath is the path where clients should POST the user JWT to notify the CS backend of changes in the current user subscription.
	SubscriptionPath = "/subscription"

	// TokenMaxSize is a safe token response size - tests with the real MS APIs suggested that those tokens will stay in between 1.2kB to 1.7kB.
	// Our Pro Token is much, much smaller.
	TokenMaxSize = 4096

	//nolint:gosec // G101 false positive, this is not a credential
	// ADTokenKey is the JSON key of the response payload of the /token endpoint.
	ADTokenKey = "azure_ad_token"
)

// SubscriptionRequest is the expected request body in json format for
// "/v1/subscription" endpoint.
//
// Must keep in sync with
// https://github.com/canonical/cloud-contracts/blob/develop/wslsaas/internal/apiv1/apiv1.go#L58
type SubscriptionRequest struct {
	// MSStoreIDKey is the user token generated on Ubuntu Pro Windows client using
	// Windows SDK.
	MSStoreIDKey string `json:"ms_store_id_key"`
}

// SyncUserSubscriptionsResponseItem is an indvidual subscription for the response to /v1/subscription.
//
// Must keep in sync with:
// https://github.com/canonical/cloud-contracts/blob/develop/wslsaas/internal/apiv1/apiv1.go#L64
type SyncUserSubscriptionsResponseItem struct {
	Token string `json:"contractToken"`
}

// SyncUserSubscriptionsResponse is the structure for json response for /v1/subscription.
//
// Must keep in sync with
// https://github.com/canonical/cloud-contracts/blob/develop/wslsaas/internal/apiv1/apiv1.go#L69
type SyncUserSubscriptionsResponse struct {
	SubscriptionEntitlements map[string]SyncUserSubscriptionsResponseItem `json:"subscriptionEntitlements"`
}
