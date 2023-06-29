// Package contractsmockserver implements a mocked version of the Contracts Server backend.
// DO NOT USE IN PRODUCTION
package contractsmockserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"path"
	"time"

	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/contracts/contractsapi"
	log "github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/grpc/logstreamer"
)

const (
	//nolint:gosec // G101 false positive, this is not a credential
	// DefaultADToken is the JSON value returned by default to the GET /token request.
	DefaultADToken = "eHy_ADToken"
	//nolint:gosec // G101 false positive, this is not a credential
	// DefaultProToken is the JSON value returned by default to the POST /susbcription request.
	DefaultProToken = "CHx_ProToken"
)

type response struct {
	value      string
	statusCode int
}

type endpointOptions struct {
	res      response
	disabled bool
	blocked  bool
}

type options struct {
	token        endpointOptions
	subscription endpointOptions
}

// Option is an optional argument for Serve.
type Option func(*options)

// WithTokenResponse sets the value of the /token endpoint response.
func WithTokenResponse(token string) Option {
	return func(o *options) {
		o.token.res.value = token
	}
}

// WithTokenStatusCode sets the /token endpoint response status code.
func WithTokenStatusCode(statusCode int) Option {
	return func(o *options) {
		o.token.res.statusCode = statusCode
	}
}

// WithSubscriptionResponse sets the value of the /subscription endpoint response.
func WithSubscriptionResponse(token string) Option {
	return func(o *options) {
		o.subscription.res.value = token
	}
}

// WithSubscriptionStatusCode sets the /subscription endpoint response status code.
func WithSubscriptionStatusCode(statusCode int) Option {
	return func(o *options) {
		o.subscription.res.statusCode = statusCode
	}
}

// WithTokenEndpointDisabled sets the option to disable the /token endpoint.
func WithTokenEndpointDisabled(disable bool) Option {
	return func(o *options) {
		o.token.disabled = disable
	}
}

// WithTokenEndpointBlocked sets the option to make the server wait forever when receiving a request to the /token endpoint.
func WithTokenEndpointBlocked(blocked bool) Option {
	return func(o *options) {
		o.token.blocked = blocked
	}
}

// WithSubscriptionEndpointDisabled sets the option to disable the /subscription endpoint.
func WithSubscriptionEndpointDisabled(disable bool) Option {
	return func(o *options) {
		o.subscription.disabled = disable
	}
}

// WithSubscriptionEndpointBlocked sets the option to make the server wait forever when receiving a request to the /susbcription endpoint.
func WithSubscriptionEndpointBlocked(blocked bool) Option {
	return func(o *options) {
		o.subscription.blocked = blocked
	}
}

// Serve starts a new HTTP server on localhost (dynamic port) mocking the Contracts Server backend REST API with responses defined according to the Option args. Cancel the ctx context to stop the server.
func Serve(ctx context.Context, args ...Option) (addr string, err error) {
	opts := options{
		token:        endpointOptions{res: response{value: DefaultADToken, statusCode: http.StatusOK}, disabled: false, blocked: false},
		subscription: endpointOptions{res: response{value: DefaultProToken, statusCode: http.StatusOK}, disabled: false, blocked: false},
	}

	for _, f := range args {
		f(&opts)
	}

	var lc net.ListenConfig
	lis, err := lc.Listen(ctx, "tcp", "localhost:")
	if err != nil {
		return "", fmt.Errorf("failed to listen over tcp: %v", err)
	}

	mux := http.NewServeMux()
	if !opts.token.disabled {
		mux.HandleFunc(path.Join(contractsapi.Version, contractsapi.TokenPath), handleTokenFunc(ctx, opts.token))
	}

	if !opts.subscription.disabled {
		mux.HandleFunc(path.Join(contractsapi.Version, contractsapi.SubscriptionPath), handleSubscriptionFunc(ctx, opts.subscription))
	}

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 3 * time.Second,
	}

	go func() {
		if err := server.Serve(lis); err != nil {
			log.Error(ctx, "failed to start the HTTP server")
		}
	}()

	return lis.Addr().String(), nil
}

// handleTokenFunc returns a a handler function for the /token endpoint according to the response options supplied.
func handleTokenFunc(ctx context.Context, o endpointOptions) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintln(w, "this endpoint only supports GET")
			return
		}

		if o.blocked {
			<-ctx.Done()
			log.Debug(ctx, "server context was cancelled, exiting...")
			return
		}

		if o.res.statusCode != 200 {
			w.WriteHeader(o.res.statusCode)
			fmt.Fprintf(w, "mock error: %d", o.res.statusCode)
			return
		}

		if _, err := fmt.Fprintf(w, `{%q: %q}`, contractsapi.ADTokenKey, o.res.value); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "failed to write the response: %v", err)
			return
		}
	}
}

// handleSubscriptionFunc returns a handler function for the /susbcription endpoint according to the response options supplied.
func handleSubscriptionFunc(ctx context.Context, o endpointOptions) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintln(w, "this endpoint only supports POST")
			return
		}

		if o.blocked {
			<-ctx.Done()
			log.Debug(ctx, "server context was cancelled, exiting...")
			return
		}

		if o.res.statusCode != 200 {
			w.WriteHeader(o.res.statusCode)
			fmt.Fprintln(w, "mock error")
			return
		}

		var data map[string]string
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintln(w, "Bad Request")
			return
		}

		userJWT, ok := data[contractsapi.JWTKey]
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintln(w, "JSON payload does not contain the expected key")
			return
		}

		if len(userJWT) == 0 {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintln(w, "JWT cannot be empty")
			return
		}

		if _, err := fmt.Fprintf(w, `{%q: %q}`, contractsapi.ProTokenKey, o.res.value); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "failed to write the response: %v", err)
			return
		}
	}
}
