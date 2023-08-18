// Package microsoftstore is a stump to allow the project to build on Linux.
package microsoftstore

import "errors"

// GenerateUserJWT takes an azure AD server access token and returns a Windows store token.
func GenerateUserJWT(azureADToken string) (string, error) {
	return "", errors.New("the windows store is not available on Linux")
}
