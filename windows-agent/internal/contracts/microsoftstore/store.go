// Package microsoftstore is a stub to implement the contract dance.
package microsoftstore

import (
	"context"
	"errors"
)

// Store is a stub type to be replaced with a real implementation.
type Store struct{}

// New creates a new client for the Microsoft Store.
func New() *Store {
	return &Store{}
}

// SubscriptionToken retrieves a JWT token for the application associated with
// the provided azureADToken.
func (*Store) SubscriptionToken(ctx context.Context, azureADToken string) (string, error) {
	return "", errors.New("the Microsoft Store interaction is not yet implemented")
}
