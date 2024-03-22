package commandservice_test

import (
	"context"
	"errors"
	"testing"

	agentapi "github.com/canonical/ubuntu-pro-for-wsl/agentapi/go"
	"github.com/canonical/ubuntu-pro-for-wsl/wsl-pro-service/internal/commandservice"
	"github.com/canonical/ubuntu-pro-for-wsl/wsl-pro-service/internal/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyProToken(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		emptyToken bool

		breakProAttach bool
		breakProDetach bool

		wantDetach bool
		wantAttach bool
		wantErr    bool
	}{
		"success attaching": {wantDetach: true, wantAttach: true},
		"success detaching": {emptyToken: true, wantDetach: true},

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

			system, mock := testutils.MockSystem(t)

			if tc.breakProAttach {
				mock.SetControlArg(testutils.ProAttachErr)
			}

			if tc.breakProDetach {
				mock.SetControlArg(testutils.ProDetachErrGeneric)
			}

			svc := commandservice.New(system)

			err := svc.ApplyProToken(context.Background(), &agentapi.ProAttachCmd{Token: token})
			if tc.wantErr {
				require.Error(t, err, "ApplyProToken call should return an error")
				return
			}
			require.NoError(t, err, "ApplyProToken should return no error")

			p := mock.Path("/.pro-detached")
			if tc.wantDetach {
				assert.FileExists(t, p, "Pro executable should have been called to pro-detach")
			} else {
				assert.NoFileExists(t, p, "Pro executable should not have been called to pro-detach")
			}

			p = mock.Path("/.pro-attached")
			if tc.wantAttach {
				assert.FileExists(t, p, "Pro executable should have been called to pro-attach")
			} else {
				assert.NoFileExists(t, p, "Pro executable should not have been called to pro-attach")
			}
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

			sys, mock := testutils.MockSystem(t)

			if tc.breakLandscapeEnable {
				mock.SetControlArg(testutils.LandscapeEnableErr)
			}

			if tc.breakLandscapeDisable {
				mock.SetControlArg(testutils.LandscapeDisableErr)
			}

			svc := commandservice.New(sys)

			err := svc.ApplyLandscapeConfig(context.Background(), &agentapi.LandscapeConfigCmd{
				Config:       config,
				HostagentUid: uid,
			})
			if tc.wantErr {
				require.Error(t, err, "ApplyLandscapeConfig call should return an error")
				return
			}
			require.NoError(t, err, "ApplyLandscapeConfig call should return no error")

			p := mock.Path("/.landscape-disabled")
			if tc.wantDisableCalled {
				assert.FileExists(t, p, "Landscape executable should have been called to disable")
			} else {
				assert.NoFileExists(t, p, "Landscape executable should not have been called to disable")
			}

			p = mock.Path("/.landscape-enabled")
			if tc.wantEnableCalled {
				assert.FileExists(t, p, "Landscape executable should have been called to enable it")
			} else {
				assert.NoFileExists(t, p, "Landscape executable should not have been called to enable it")
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

func TestWithProMock(t *testing.T)             { testutils.ProMock(t) }
func TestWithLandscapeConfigMock(t *testing.T) { testutils.LandscapeConfigMock(t) }
func TestWithWslPathMock(t *testing.T)         { testutils.WslPathMock(t) }
func TestWithWslInfoMock(t *testing.T)         { testutils.WslInfoMock(t) }
func TestWithCmdExeMock(t *testing.T)          { testutils.CmdExeMock(t) }