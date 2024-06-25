package tasks_test

import (
	"context"
	"errors"
	"testing"

	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/tasks"
	"github.com/stretchr/testify/require"
)

//nolint:dupl // Those tests are very similar because the tasks and their failure modes are, but yet not the same.
func TestProAttachment(t *testing.T) {
	testcases := map[string]struct {
		token string

		wantErr bool
	}{
		"Success": {},

		"Error when the connection fails to send a task": {token: "MOCK_ERROR", wantErr: true},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			switch tc.token {
			case "":
				tc.token = "Good Token"
			case "-":
				tc.token = ""
			default:
			}
			// Create a new ProAttachment task.
			proAttachment := tasks.ProAttachment{
				Token: tc.token,
			}

			conn := mockConnection{}
			err := proAttachment.Execute(context.Background(), conn)
			if tc.wantErr {
				require.Error(t, err, "Execute should have failed")
			} else {
				require.NoError(t, err, "Execute should have succeeded")
			}

			// Comparison and stringyfication
			another := tasks.ProAttachment{Token: "another token"}
			require.True(t, proAttachment.Is(another), "All ProAttachment tasks should be considered equivalent")
			require.NotContains(t, proAttachment.String(), tc.token, "ProAttachment.String should not reveal the complete token")
		})
	}
}

//nolint:dupl // Those tests are very similar because the tasks and their failure modes are, but yet not the same.
func TestLandscapeConfigure(t *testing.T) {
	testcases := map[string]struct {
		config string

		wantErr bool
	}{
		"Success": {},

		"Error when the connection fails to send a task": {config: "MOCK_ERROR", wantErr: true},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			switch tc.config {
			case "":
				tc.config = "[client]\nkey = value"
			case "-":
				tc.config = ""
			default:
			}
			// Create a new LandscapeConfigure task.
			landscapeConfigure := tasks.LandscapeConfigure{
				Config: tc.config,
			}

			conn := mockConnection{}
			err := landscapeConfigure.Execute(context.Background(), conn)
			if tc.wantErr {
				require.Error(t, err, "Execute should have failed")
			} else {
				require.NoError(t, err, "Execute should have succeeded")
			}

			// Comparison and stringyfication
			another := tasks.LandscapeConfigure{Config: "another configuration"}
			require.True(t, landscapeConfigure.Is(another), "All LandscapeConfigure tasks should be considered equivalent")
			require.NotContains(t, landscapeConfigure.String(), tc.config, "LandscapeConfigure.String should not reveal the contents of the configuration")
		})
	}
}

type mockConnection struct{}

func (m mockConnection) SendProAttachment(proToken string) error {
	switch proToken {
	case "MOCK_ERROR":
		return errors.New("mock error")
	default:
		return nil
	}
}

func (m mockConnection) SendLandscapeConfig(lpeConfig string) error {
	switch lpeConfig {
	case "MOCK_ERROR":
		return errors.New("mock error")
	default:
		return nil
	}
}
