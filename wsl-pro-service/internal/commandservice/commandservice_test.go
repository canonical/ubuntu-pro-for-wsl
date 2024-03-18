package commandservice_test

import (
	"context"
	"errors"
	"testing"

	agentapi "github.com/canonical/ubuntu-pro-for-wsl/agentapi/go"
	"github.com/canonical/ubuntu-pro-for-wsl/wsl-pro-service/internal/commandservice"
	"github.com/stretchr/testify/require"
)

func TestApplyProToken(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		emptyToken bool

		breakProAttach bool
		breakProDetach bool

		wantErr    bool
		wantAttach bool
	}{
		"success attaching": {wantAttach: true},
		"success detaching": {emptyToken: true},

		// Attach/detach errors
		"Error calling pro detach": {breakProDetach: true, wantErr: true},
		"Error calling pro attach": {breakProAttach: true, wantErr: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			token := "123abc"
			if tc.emptyToken {
				token = ""
			}

			system := &mockSystem{
				breakProAttach: tc.breakProAttach,
				breakProDetach: tc.breakProDetach,
			}

			svc := commandservice.New(system)

			err := svc.ApplyProToken(context.Background(), &agentapi.ProAttachCmd{Token: token})
			if tc.wantErr {
				require.Error(t, err, "ApplyProToken call should return an error")
				return
			}
			require.NoError(t, err, "ApplyProToken call should return no error")

			require.Equal(t, 1, system.proDetachCalled, "ProDetach should have been called exactly once")

			if !tc.wantAttach {
				require.Empty(t, system.proAttachCalled, "ProAttach should not have been called")
				return
			}
			require.Len(t, system.proAttachCalled, 1, "ProAttach should have been called exactly once")
			require.Equal(t, token, system.proAttachCalled[0], "Mismatch between submitted token and the one used to attach")
		})
	}
}

func TestApplyLandscapeConfig(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		emptyConfig bool

		breakLandscapeEnable  bool
		breakLandscapeDisable bool

		wantErr bool

		wantDisableCalled bool
		wantEnableCalled  bool
	}{
		"success enabling Landscape":  {wantEnableCalled: true},
		"success disabling Landscape": {emptyConfig: true, wantDisableCalled: true},

		// Attach/detach errors
		"Error calling landscape disable": {emptyConfig: true, breakLandscapeDisable: true, wantErr: true},
		"Error calling landscape enable":  {breakLandscapeEnable: true, wantErr: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			uid := "this-is-a-uid"
			config := "[client]\nhello=world"
			if tc.emptyConfig {
				config = ""
			}

			system := &mockSystem{
				breakLandscapeEnable:  tc.breakLandscapeEnable,
				breakLandscapeDisable: tc.breakLandscapeDisable,
			}

			svc := commandservice.New(system)

			err := svc.ApplyLandscapeConfig(context.Background(), &agentapi.LandscapeConfigCmd{
				Config:       config,
				HostagentUid: uid,
			})
			if tc.wantErr {
				require.Error(t, err, "ApplyLandscapeConfig call should return an error")
				return
			}
			require.NoError(t, err, "ApplyLandscapeConfig call should return no error")

			if tc.wantDisableCalled {
				require.Equal(t, 1, system.landscapeDisableCalled, "LandscapeDisable should have been called exactly once")
			} else {
				require.Zero(t, system.landscapeDisableCalled, "LandscapeDisable should not have been called")
			}

			if tc.wantEnableCalled {
				require.Len(t, system.landscapeEnableCalled, 1, "LandsscapeEnable should have been called exactly once")
				require.Equal(t, config, system.landscapeEnableCalled[0].config, "Mismatch between submitted config and the one used to attach")
				require.Equal(t, uid, system.landscapeEnableCalled[0].uid, "Mismatch between submitted hostagent UID and the one used to attach")
			} else {
				require.Empty(t, system.landscapeEnableCalled, "LandsscapeEnable should not have been called")
			}
		})
	}
}

type mockSystem struct {
	breakProAttach        bool
	breakProDetach        bool
	breakLandscapeEnable  bool
	breakLandscapeDisable bool

	proAttachCalled       []string
	proDetachCalled       int
	landscapeEnableCalled []struct {
		config string
		uid    string
	}
	landscapeDisableCalled int
}

func (m *mockSystem) ProAttach(ctx context.Context, token string) error {
	if m.breakProAttach {
		return errors.New("ProAttach: mock error")
	}

	m.proAttachCalled = append(m.proAttachCalled, token)

	return nil
}

func (m *mockSystem) ProDetach(ctx context.Context) error {
	if m.breakProDetach {
		return errors.New("ProDetach: mock error")
	}

	m.proDetachCalled++

	return nil
}

func (m *mockSystem) LandscapeEnable(ctx context.Context, conf, uid string) error {
	if m.breakLandscapeEnable {
		return errors.New("LandscapeEnable: mock error")
	}

	m.landscapeEnableCalled = append(m.landscapeEnableCalled, struct {
		config string
		uid    string
	}{conf, uid})

	return nil
}

func (m *mockSystem) LandscapeDisable(ctx context.Context) error {
	if m.breakLandscapeDisable {
		return errors.New("LandscapeDisable: mock error")
	}

	m.landscapeDisableCalled++

	return nil
}
