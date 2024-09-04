package daemon_test

import (
	"context"
	"errors"
	"maps"
	"testing"
	"time"

	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/daemon"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/daemon/daemontestutils"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/daemon/netmonitoring"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestSubscribe(t *testing.T) {
	t.Parallel()

	mockError := errors.New("mock error")

	before := map[string]string{
		uuid.New().String(): "conn0",
		uuid.New().String(): "conn1",
		uuid.New().String(): "conn2",
	}

	after := map[string]string{
		uuid.New().String(): "new",
		"not a guid":        "yet_another_new",
	}

	maps.Copy(after, before)

	testcases := map[string]struct {
		initError             error
		listDevicesError      error
		listDevicesAfterError error
		getConnNameError      error
		waitError             error

		ctxCancel           bool
		startWithNoAdapters bool
		devicesUnchanged    bool

		wantErr        bool
		wantNoCallback bool
		wantName       string
	}{
		"Success": {},
		"When the system starts with no adapters": {startWithNoAdapters: true, wantName: "conn0"},

		"Cannot subscribe when initializing the API fails":    {initError: mockError, wantErr: true},
		"Cannot subscribe when listing devices fails":         {listDevicesError: mockError, wantErr: true},
		"Cannot subscribe when getting connection name fails": {getConnNameError: mockError, wantErr: true},

		"Cannot notify when waiting for changes fails":                         {waitError: mockError, wantNoCallback: true},
		"Cannot notify when the context is cancelled while waiting":            {ctxCancel: true, wantNoCallback: true},
		"Cannot notify when OS triggers a notification without device changes": {devicesUnchanged: true, wantNoCallback: true},
		"Cannot notify when listing devices on notification fails":             {listDevicesAfterError: mockError, wantNoCallback: true},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			switch tc.wantName {
			case "":
				tc.wantName = "new"
			case "-":
				tc.wantName = ""
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			initAPI := func() (netmonitoring.DevicesAPI, error) {
				if tc.initError != nil {
					return nil, tc.initError
				}

				b := before
				if tc.startWithNoAdapters {
					b = make(map[string]string, 0)
				}
				a := after
				if tc.devicesUnchanged {
					a = before
				}
				return &daemontestutils.NetMonitoringMockAPI{
					Before:                       b,
					After:                        a,
					ListDevicesError:             tc.listDevicesError,
					ListDevicesAfterError:        tc.listDevicesAfterError,
					GetDeviceConnectionNameError: tc.getConnNameError,
					WaitForDeviceChangesImpl: func() error {
						if tc.ctxCancel {
							cancel()
							<-ctx.Done()
							return ctx.Err()
						}
						// Introduces some asynchrony to the test.
						<-time.After(50 * time.Millisecond)
						return tc.waitError
					},
				}, nil
			}

			added := make(chan string, 1)
			defer close(added)
			callback := func(adapterNames []string) bool {
				added <- adapterNames[0]
				return false
			}

			n, err := daemon.Subscribe(ctx, callback, daemon.WithNetDevicesAPIProvider(initAPI))

			if tc.wantErr {
				require.Error(t, err, "Subscribe should have failed")
				return
			}
			require.NoError(t, err, "Subscribe should have succeeded")

			select {
			case res := <-added:
				require.Equal(t, tc.wantName, res, "unexpected new network adapter")
			case <-time.After(200 * time.Millisecond):
				if !tc.wantNoCallback {
					require.Fail(t, "timeout waiting for new network adapter")
				}
			}

			// Collect the error reported by the wait operation.
			err = n.Stop()
			if tc.waitError != nil || tc.wantNoCallback || tc.ctxCancel {
				require.Error(t, err, "Stop should have failed")
				return
			}
			require.NoError(t, err, "Stop should have succeeded")
		})
	}
}
