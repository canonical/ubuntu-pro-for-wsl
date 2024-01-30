// Package storemockserver implements a mocked version of the Windows Runtime components involved in the MS Store API that talks via REST.
// DO NOT USE IN PRODUCTION
package storemockserver

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/canonical/ubuntu-pro-for-wsl/mocks/restserver"
	"golang.org/x/exp/slices"
)

const (
	// endpoint paths.

	//AllAuthenticatedUsersPath is the path to GET the list of anonymous user ID's locally authenticated.
	AllAuthenticatedUsersPath = "/allauthenticatedusers"

	// GenerateUserJWTPath is the path to GET the user's store ID key (a.k.a. JWT).
	GenerateUserJWTPath = "/generateuserjwt"

	// ProductPath is the path to GET a collection of products related to the current application.
	ProductPath = "/products"

	// PurchasePath is the path to GET to purchase a subscription.
	PurchasePath = "/purchase"

	// endpoint URL parameter keys.

	// ProductIDParam is the URL encoded key parameter to the product ID.
	ProductIDParam = "id"

	// ProductIDsParam is the plural version of the above for retrieving a collection of products associated with the current application.
	ProductIDsParam = "ids"

	// ProductKindsParam is the URL encoded key parameter to filter the collection of products associated with the current application.
	ProductKindsParam = "kinds"

	// ServiceTicketParam is the URL encoded key parameter to the service ticket input to generate the user JWT (a.k.a. the Azure AD token).
	ServiceTicketParam = "serviceticket"

	// PublisherUserIDParam is the URL encoded key parameter to the anonymous user ID to be encoded in the JWT (a.k.a. the user ID).
	PublisherUserIDParam = "publisheruserid"

	// predefined error triggering inputs.

	// CannotPurchaseValue is the product ID that triggers a product purchase error.
	CannotPurchaseValue = "cannotpurchase"

	// ExpiredTokenValue is a token input that triggers the expired AAD token error.
	ExpiredTokenValue = "expiredtoken"

	// NonExistentValue is the product ID that triggers a product not found error.
	NonExistentValue = "nonexistent"

	// ServerErrorValue is the product ID and service ticket inputs that triggers an internal server error.
	ServerErrorValue = "servererror"

	// Purchase result values
	// https://learn.microsoft.com/en-us/uwp/api/windows.services.store.storepurchasestatus?view=winrt-22621#fields
	// "NetworkError" is technically not needed, since this is a client-originated error.

	// AlreadyPurchasedResult is the response value from the purchase endpoint when the user has previously purchased the supplied product ID.
	AlreadyPurchasedResult = "AlreadyPurchased"

	// NotPurchasedResult is the response value from the purchase endpoint when not even the store known why it failed :) .
	NotPurchasedResult = "NotPurchased"

	// ServerErrorResult is the response value from the purchase endpoint when an internal server error happens.
	ServerErrorResult = "ServerError"

	// SucceededResult is the response value of a succesfull purchase.
	SucceededResult = "Succeeded"

	// JSON response schema.

	// UsersResponseKey is the JSON key of the response containing the list of locally authenticated users.
	UsersResponseKey = "users"

	// JWTResponseKey is the JSON key of the user JWT response.
	JWTResponseKey = "jwt"

	// PurchaseStatusKey is the JSON key of the purchase status response.
	PurchaseStatusKey = "status"
)

// Settings contains the parameters for the Server.
type Settings struct {
	AllAuthenticatedUsers restserver.Endpoint
	GenerateUserJWT       restserver.Endpoint
	GetProducts           restserver.Endpoint
	Purchase              restserver.Endpoint

	AllProducts []Product
}

// Unmarshal tricks the type system so marshalling YAML will just work when called from the restserver.Settings interface.
func (s Settings) Unmarshal(in []byte, unmarshaller func(in []byte, out interface{}) (err error)) (restserver.Settings, error) {
	err := unmarshaller(in, &s)
	return s, err
}

// Server is a configurable mock of the MS Store runtime component that talks REST.
type Server struct {
	restserver.ServerBase
	settings Settings

	settingsMu sync.RWMutex
}

// Product models the interesting properties from the MS StoreProduct type.
type Product struct {
	StoreID            string
	Title              string
	Description        string
	IsInUserCollection bool
	ProductKind        string
	ExpirationDate     time.Time
}

// DefaultSettings returns the default set of Settings for the server.
func DefaultSettings() Settings {
	return Settings{
		AllProducts:           []Product{{StoreID: "A_NICE_ID", Title: "A nice title", Description: "A nice description", IsInUserCollection: false, ProductKind: "Durable", ExpirationDate: time.Time{}}},
		AllAuthenticatedUsers: restserver.Endpoint{OnSuccess: restserver.Response{Value: `"user@email.pizza"`, Status: http.StatusOK}},
		GenerateUserJWT:       restserver.Endpoint{OnSuccess: restserver.Response{Value: "AMAZING_JWT", Status: http.StatusOK}},
		// Predefined success configuration for those endpoints doesn't really make sense.
		GetProducts: restserver.NewEndpoint(),
		Purchase:    restserver.NewEndpoint(),
	}
}

// NewServer creates a new store mock server with the provided Settings.
func NewServer(s Settings) *Server {
	sv := &Server{
		settings: s,
	}

	mux := http.NewServeMux()

	if !s.AllAuthenticatedUsers.Disabled {
		mux.HandleFunc(AllAuthenticatedUsersPath, sv.generateHandler(s.AllAuthenticatedUsers, sv.handleAllAuthenticatedUsers))
	}

	if !s.GenerateUserJWT.Disabled {
		mux.HandleFunc(GenerateUserJWTPath, sv.generateHandler(s.GenerateUserJWT, sv.handleGenerateUserJWT))
	}

	if !s.GetProducts.Disabled {
		mux.HandleFunc(ProductPath, sv.generateHandler(s.GetProducts, sv.handleGetProducts))
	}

	if !s.Purchase.Disabled {
		mux.HandleFunc(PurchasePath, sv.generateHandler(s.Purchase, sv.handlePurchase))
	}

	sv.Mux = mux

	return sv
}

// Generates a request handler function by chaining calls to the server request validation routine and the actual handler.
func (s *Server) generateHandler(endpoint restserver.Endpoint, handler func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := s.ValidateRequest(w, r, http.MethodGet, endpoint); err != nil {
			fmt.Fprintf(w, "%v", err)
			return
		}

		handler(w, r)
	}
}

// Handlers

func (s *Server) handleAllAuthenticatedUsers(w http.ResponseWriter, r *http.Request) {
	resp := s.settings.AllAuthenticatedUsers.OnSuccess
	fmt.Fprintf(w, `{%q:[%s]}`, UsersResponseKey, resp.Value)
}

func (s *Server) handleGenerateUserJWT(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	// https://learn.microsoft.com/en-us/uwp/api/windows.services.store.storecontext.getcustomerpurchaseidasync
	serviceTicket := q.Get(ServiceTicketParam)
	publisherUserID := q.Get(PublisherUserIDParam)
	if len(serviceTicket) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "service ticket (Azure access token) is required.")
		return
	}

	// Predefined errors
	if serviceTicket == ExpiredTokenValue {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "service ticket is expired.")
		return
	}

	if serviceTicket == ServerErrorValue {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "internal server error.")
		return
	}

	responseValue := s.settings.GenerateUserJWT.OnSuccess.Value
	// The user JWT may encode an anonymous ID that identifies the current user in the context of services that manage the current app.
	if len(publisherUserID) > 0 {
		responseValue += "_from_user_" + publisherUserID
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	fmt.Fprintf(w, `{%q:%q}`, JWTResponseKey, responseValue)
}

func (s *Server) handleGetProducts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	kinds := q[ProductKindsParam]
	ids := q[ProductIDsParam]
	var productsFound []Product

	// To avoid race with handlePurchase
	s.settingsMu.RLock()
	defer s.settingsMu.RUnlock()

	for _, p := range s.settings.AllProducts {
		if slices.Contains(kinds, p.ProductKind) && slices.Contains(ids, p.StoreID) {
			productsFound = append(productsFound, p)
		}
	}

	slog.Info(fmt.Sprintf("products found: %v", productsFound))
	bs, err := json.Marshal(productsFound)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "failed to marshall the matching products: %v", err)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	fmt.Fprintf(w, `{%q:%s}`, "products", string(bs))
}

func (s *Server) handlePurchase(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get(ProductIDParam)

	if len(id) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "%s is required.", ProductIDParam)
		return
	}

	if id == NonExistentValue {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "product %s does not exist", id)
		return
	}

	if id == ServerErrorValue {
		slog.Info(fmt.Sprintf("%s: server error triggered. Product ID was: %s", PurchasePath, id))
		fmt.Fprintf(w, `{%q:%q}`, PurchaseStatusKey, ServerErrorResult)
		return
	}

	if id == CannotPurchaseValue {
		slog.Info(fmt.Sprintf("%s: purchase error triggered. Product ID was: %s", PurchasePath, id))
		fmt.Fprintf(w, `{%q:%q}`, PurchaseStatusKey, NotPurchasedResult)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	s.settingsMu.Lock()
	defer s.settingsMu.Unlock()

	for i, p := range s.settings.AllProducts {
		if p.StoreID != id {
			continue
		}

		if p.IsInUserCollection {
			slog.Info(fmt.Sprintf("%s: product %q already in user collection", PurchasePath, id))
			fmt.Fprintf(w, `{%q:%q}`, PurchaseStatusKey, AlreadyPurchasedResult)
			return
		}

		year, month, day := time.Now().Date()

		s.settings.AllProducts[i].ExpirationDate = time.Date(year+1, month, day, 1, 1, 1, 1, time.Local) // one year from now.
		s.settings.AllProducts[i].IsInUserCollection = true
		fmt.Fprintf(w, `{%q:%q}`, PurchaseStatusKey, SucceededResult)
		return
	}

	w.WriteHeader(http.StatusBadRequest)
	fmt.Fprintf(w, "product %s does not exist", id)
}
