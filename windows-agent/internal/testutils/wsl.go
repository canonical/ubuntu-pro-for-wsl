package testutils

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

const testDistroPrefix = "testDistro_UP4W"
const testDistroPattern = "%s_%s_%d"

// NonRegisteredDistro generates a random distroName and GUID but does not register them.
func NonRegisteredDistro(t *testing.T) (distroName string, GUID string) {
	t.Helper()

	distroName = RandomDistroName(t)

	guid, err := uuid.NewRandom()
	require.NoError(t, err, "Setup: could not generate a GUID for the non-registered distro")

	GUID = fmt.Sprintf("{%s}", guid.String())
	return distroName, GUID
}

// RandomDistroName generates a distroName that is not registered.
func RandomDistroName(t *testing.T) (name string) {
	t.Helper()

	testFullNormalized := normalizeName(t, strings.ReplaceAll(t.Name(), "/", "--"))

	//nolint: gosec // No need to be cryptographically secure
	return fmt.Sprintf(testDistroPattern, testDistroPrefix, testFullNormalized, rand.Uint64())
}

// Avoids misuse of testutils.
func requireIsTestDistro(t *testing.T, distroName string) {
	t.Helper()

	if !strings.HasPrefix(distroName, testDistroPrefix) {
		require.Fail(t, "testutils can only be used with test distros", "Requested distro: %s", distroName)
	}
}
