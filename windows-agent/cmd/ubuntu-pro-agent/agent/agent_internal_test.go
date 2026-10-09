// Internal (white-box) tests for the agent. The serve path's early error
// branches are unreachable through the public Run flow because RunE resolves
// the same paths first, so they are driven here by calling serve directly.

package agent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestServeErrors(t *testing.T) {
	// Not parallel: t.Setenv forbids it.

	testCases := map[string]struct {
		// withPublicDir gives the run a resolvable public directory.
		withPublicDir bool

		unsetUserProfile  bool
		unsetLocalAppData bool
	}{
		"Error when the public directory path cannot be resolved":  {unsetUserProfile: true},
		"Error when the private directory path cannot be resolved": {withPublicDir: true, unsetLocalAppData: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			if tc.unsetUserProfile {
				t.Setenv("UserProfile", "")
			}
			if tc.unsetLocalAppData {
				t.Setenv("LocalAppData", "")
			}

			var opt options
			if tc.withPublicDir {
				opt.publicDir = t.TempDir()
			}

			a := New()
			err := a.serve(context.Background(), opt)
			require.Error(t, err)
		})
	}
}
