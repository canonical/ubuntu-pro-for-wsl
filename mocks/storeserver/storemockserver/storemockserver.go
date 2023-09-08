// Package storemockserver implements a mocked version of the Windows Runtime components involved in the MS Store API that talks via REST.
// DO NOT USE IN PRODUCTION
package storemockserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/canonical/ubuntu-pro-for-windows/mocks/restserver"
	"golang.org/x/exp/slices"
	"golang.org/x/exp/slog"
)

// Settings contains the parameters for the Server.
type Settings struct {
	AllAuthenticatedUsers restserver.Endpoint
	GenerateUserJWT       restserver.Endpoint
	GetProducts           restserver.Endpoint
	Purchase              restserver.Endpoint

	AllProducts []Product

	address string
}

// Server is a configurable mock of the MS Store runtime component that talks REST.
type Server struct {
	restserver.ServerBase
	settings Settings
}

// SetAddress updates a Settings object with the new address.
func (s *Settings) SetAddress(address string) {
	s.address = address
}

// GetAddress returns the previously set address.
func (s *Settings) GetAddress() string {
	return s.address
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
		address:               "localhost:0",
		AllProducts:           []Product{{StoreID: "A_NICE_ID", Title: "A nice title", Description: "A nice description", IsInUserCollection: false, ProductKind: "Durable", ExpirationDate: time.Time{}}},
		AllAuthenticatedUsers: restserver.Endpoint{OnSuccess: restserver.Response{Value: `"user@email.pizza"`, Status: http.StatusOK}},
		GenerateUserJWT:       restserver.Endpoint{OnSuccess: restserver.Response{Value: "AMAZING_JWT", Status: http.StatusOK}},
		// Predefined success configuration for those endpoints doesn't really make sense.
		GetProducts: restserver.EndpointOk(),
		Purchase:    restserver.EndpointOk(),
	}
}

// NewServer creates a new store mock server with the provided Settings.
func NewServer(s Settings) *Server {
	sv := &Server{
		ServerBase: restserver.ServerBase{GetAddress: s.GetAddress},
		settings:   s,
	}
	sv.Mux = sv.NewMux()

	return sv
}

// NewMux sets up a ServeMux to handle the server endpoints enabled according to the server settings.
func (s *Server) NewMux() *http.ServeMux {
	mux := http.NewServeMux()

	if !s.settings.AllAuthenticatedUsers.Disabled {
		mux.HandleFunc("/allauthenticatedusers", s.generateHandler(s.settings.AllAuthenticatedUsers, s.handleAllAuthenticatedUsers))
	}

	if !s.settings.GenerateUserJWT.Disabled {
		mux.HandleFunc("/generateuserjwt", s.generateHandler(s.settings.GenerateUserJWT, s.handleGenerateUserJWT))
	}

	if !s.settings.GetProducts.Disabled {
		mux.HandleFunc("/products", s.generateHandler(s.settings.GetProducts, s.handleGetProducts))
	}

	if !s.settings.Purchase.Disabled {
		mux.HandleFunc("/purchase", s.generateHandler(s.settings.Purchase, s.handlePurchase))
	}

	return mux
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
	fmt.Fprintf(w, `{"users":[%s]}`, resp.Value)
}

func (s *Server) handleGenerateUserJWT(w http.ResponseWriter, r *http.Request) {
	resp := s.settings.GenerateUserJWT.OnSuccess
	if resp.Status != http.StatusOK {
		w.WriteHeader(resp.Status)
		fmt.Fprintf(w, "mock error: %d", resp.Status)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	fmt.Fprintf(w, `{"jwt":%q}`, resp.Value)
}

func (s *Server) handleGetProducts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	kinds := q["kinds"]
	ids := q["ids"]
	var productsFound []Product
	for _, p := range s.settings.AllProducts {
		if slices.Contains(kinds, p.ProductKind) && slices.Contains(ids, p.StoreID) {
			productsFound = append(productsFound, p)
		}
	}

	bs, err := json.Marshal(productsFound)
	if err != nil {
		fmt.Fprintf(w, "failed to marshall the matching products: %v", err)
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	fmt.Fprint(w, string(bs))
}

// https://learn.microsoft.com/en-us/uwp/api/windows.services.store.storepurchasestatus?view=winrt-22621#fields
const (
	// "NetworkError" is technically not needed, since this is a client-originated error.
	alreadyPurchased = "AlreadyPurchased"
	notPurchased     = "NotPurchased"
	serverError      = "ServerError"
	succeeded        = "Succeeded"
)

func (s *Server) handlePurchase(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")

	if len(id) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "product ID is required.")
		return
	}

	if id == "nonexistent" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "product %s does not exist", id)
		return
	}

	if id == "servererror" {
		slog.Info("server error triggered", id)
		fmt.Fprintf(w, `{"status":%q}`, serverError)
		return
	}

	if id == "cannotpurchase" {
		slog.Info("purchase error triggered", id)
		fmt.Fprintf(w, `{"status":%q}`, notPurchased)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	for i, p := range s.settings.AllProducts {
		if p.StoreID != id {
			continue
		}

		if p.IsInUserCollection {
			slog.Info("product already in user collection", id)
			fmt.Fprintf(w, `{"status":%q}`, alreadyPurchased)
			return
		}

		year, month, day := time.Now().Date()
		s.settings.AllProducts[i].ExpirationDate = time.Date(year+1, month, day, 1, 1, 1, 1, time.Local) // one year from now.
		s.settings.AllProducts[i].IsInUserCollection = true
		fmt.Fprintf(w, `{"status":%q}`, succeeded)
		return
	}

	w.WriteHeader(http.StatusBadRequest)
	fmt.Fprintf(w, "product %s does not exist", id)
}
