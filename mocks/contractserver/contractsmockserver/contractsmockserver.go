// Package contractsmockserver implements a mocked version of the Contracts Server backend.
// DO NOT USE IN PRODUCTION
package contractsmockserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path"

	"github.com/canonical/ubuntu-pro-for-windows/common"
	"github.com/canonical/ubuntu-pro-for-windows/contractsapi"
	"github.com/canonical/ubuntu-pro-for-windows/mocks/restserver"
)

const (
	//nolint:gosec // G101 false positive, this is not a credential
	// DefaultADToken is the value returned by default to the GET /token request, encoded in a JSON object.
	DefaultADToken = "eHy_ADToken"
	//nolint:gosec // G101 false positive, this is not a credential
	// DefaultProToken is the value returned by default to the POST /susbcription request, encoded in a JSON object.
	DefaultProToken = "CHx_ProToken"
)

// Server is a mock of the contract server, where its behaviour can be modified.
type Server struct {
	restserver.ServerBase
	settings Settings
}

// Settings contains the parameters for the Server.
type Settings struct {
	Token        restserver.Endpoint
	Subscription restserver.Endpoint
}

// Unmarshal tricks the type system so marshalling YAML will just work when called from the restserver.Settings interface.
func (s Settings) Unmarshal(in []byte, unmarshaller func(in []byte, out interface{}) (err error)) (restserver.Settings, error) {
	err := unmarshaller(in, &s)
	return s, err
}

// DefaultSettings returns the default set of settings for the server.
func DefaultSettings() Settings {
	return Settings{
		Token:        restserver.Endpoint{OnSuccess: restserver.Response{Value: DefaultADToken, Status: http.StatusOK}},
		Subscription: restserver.Endpoint{OnSuccess: restserver.Response{Value: DefaultProToken, Status: http.StatusOK}},
	}
}

// NewServer creates a new contract server with the provided settings.
func NewServer(s Settings) *Server {
	sv := &Server{settings: s}
	mux := http.NewServeMux()

	if !s.Token.Disabled {
		mux.HandleFunc(path.Join(contractsapi.Version, contractsapi.TokenPath), sv.handleToken)
	}

	if !s.Subscription.Disabled {
		mux.HandleFunc(path.Join(contractsapi.Version, contractsapi.SubscriptionPath), sv.handleSubscription)
	}
	sv.Mux = mux

	return sv
}

// handleToken implements the /token endpoint.
func (s *Server) handleToken(w http.ResponseWriter, r *http.Request) {
	if err := s.ValidateRequest(w, r, http.MethodGet, s.settings.Token); err != nil {
		fmt.Fprintf(w, "%v", err)
		return
	}

	if _, err := fmt.Fprintf(w, `{%q: %q}`, contractsapi.ADTokenKey, s.settings.Token.OnSuccess.Value); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "failed to write the response: %v", err)
		return
	}
}

// handleSubscription implements the /susbcription endpoint.
func (s *Server) handleSubscription(w http.ResponseWriter, r *http.Request) {
	if err := s.ValidateRequest(w, r, http.MethodPost, s.settings.Subscription); err != nil {
		fmt.Fprintf(w, "%v", err)
		return
	}

	var req contractsapi.SubscriptionRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, "Bad Request")
		return
	}

	if req.MSStoreIDKey == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, "JWT cannot be empty")
		return
	}

	// In the server, the ID for the product is "ProductID:SKU".
	// Here we choose some arbitrary number for the SKU.
	id := common.MsStoreProductID + ":0001"

	resp := contractsapi.SyncUserSubscriptionsResponse{
		SubscriptionEntitlements: map[string]contractsapi.SyncUserSubscriptionsResponseItem{
			id:             {Token: s.settings.Subscription.OnSuccess.Value},
			"ABCDEFGHIJKL": {Token: "a-token-for-some-other-subscription"},
		},
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "failed to write the response: %v", err)
		return
	}
}
