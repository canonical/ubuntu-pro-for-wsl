package testutils

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

const testDistroPrefix = "testDistro_UP4W"
const testDistroPattern = "%s_%s_%d"

// RandomDistroName generates a distroName that is not registered.
func RandomDistroName(t *testing.T) (name string) {
	t.Helper()

	testFullNormalized := normalizeName(t, strings.ReplaceAll(t.Name(), "/", "--"))

	//nolint: gosec // No need to be cryptographically secure
	return fmt.Sprintf(testDistroPattern, testDistroPrefix, testFullNormalized, rand.Uint64())
}
