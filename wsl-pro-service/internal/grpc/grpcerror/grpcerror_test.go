package grpcerror_test

import (
	"errors"
	"testing"

	"github.com/canonical/ubuntu-pro-for-wsl/wsl-pro-service/internal/grpc/grpcerror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestFormat(t *testing.T) {
	t.Parallel()

	const errMSG = "foo"

	tests := map[string]struct {
		err error

		wantNilError          bool
		wantStatusName        string
		wantDaemonName        bool
		wantOverriddenMessage bool
	}{
		"Non GRPC errors are returned as is": {err: errors.New(errMSG)},
		"Nil returns nil":                    {err: nil, wantNilError: true},

		"GRPC Unavailable errors prints daemon name":                     {err: status.Error(codes.Unavailable, errMSG), wantDaemonName: true},
		"GRPC Deadline errors don't print status nor daemon nor message": {err: status.Error(codes.DeadlineExceeded, errMSG), wantOverriddenMessage: true},
		"GRPC Unknown errors don't print status and daemon":              {err: status.Error(codes.Unknown, errMSG)},
		"GRPC Random errors prints status and message":                   {err: status.Error(codes.Internal, errMSG)},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			tc := tc
			t.Parallel()

			err := grpcerror.Format(tc.err, "DaemonName")

			if tc.wantNilError {
				require.NoError(t, err, "Error should be nil when input is nil")
				return
			}

			_, grpcError := status.FromError(err)
			require.False(t, grpcError, "Error is not a GRPC error")

			if tc.wantDaemonName {
				assert.Contains(t, err.Error(), "DaemonName", "Daemon name is in error")
			} else {
				assert.NotContains(t, err.Error(), "DaemonName", "Daemon name is not in error")
			}

			assert.Contains(t, err.Error(), tc.wantStatusName, "Status name is contained in error or empty")

			if tc.wantOverriddenMessage {
				assert.NotContains(t, err.Error(), errMSG, "Real error message is not printed")
			} else {
				assert.Contains(t, err.Error(), errMSG, "Real error message is printed")
			}
		})
	}
}
