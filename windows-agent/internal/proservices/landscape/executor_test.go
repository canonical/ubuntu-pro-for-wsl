package landscape_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	landscapeapi "github.com/canonical/landscape-hostagent-api"
	"github.com/canonical/ubuntu-pro-for-wsl/common/wsltestutils"
	"github.com/canonical/ubuntu-pro-for-wsl/mocks/landscape/landscapemockservice"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/consts"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/database"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/distros/distro"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/proservices/landscape"
	"github.com/stretchr/testify/require"
	wsl "github.com/ubuntu/gowsl"
	wslmock "github.com/ubuntu/gowsl/mock"
)

const (
	testAppx       = "CanonicalGroupLimited.Ubuntu22.04LTS" // The name of the Appx
	testDistroAppx = "Ubuntu-22.04"                         // The name used in `wsl --install <DISTRO>`
)

func TestAssignHost(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		confErr bool
		uid     string
		wantErr bool
	}{
		"With some uid": {uid: "HostUID123"},

		"Error with an empty uid":            {uid: "", wantErr: true},
		"Error when config returns an error": {confErr: true, wantErr: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			testReceiveCommand(t, distroSettings{}, t.TempDir(), t.TempDir(),
				// Test setup
				func(testBed *commandTestBed) *landscapeapi.Command {
					if tc.confErr {
						testBed.conf.setLandscapeUIDErr = true
					}

					return &landscapeapi.Command{
						Cmd: &landscapeapi.Command_AssignHost_{AssignHost: &landscapeapi.Command_AssignHost{Uid: tc.uid}},
					}
				},
				// Test assertions
				func(testBed *commandTestBed) {
					const maxTimeout = time.Second
					if tc.wantErr {
						time.Sleep(maxTimeout)
						require.NotEqual(t, tc.uid, testBed.conf.landscapeAgentUID, "Landscape UID should not have been assigned")
						return
					}

					require.Eventually(t, func() bool {
						testBed.conf.mu.Lock()
						defer testBed.conf.mu.Unlock()

						return testBed.conf.landscapeAgentUID == tc.uid
					}, maxTimeout, 100*time.Millisecond, "Landscape client should have overridden the initial UID sent by the server")
				})
		})
	}
}

func TestReceiveCommandStartStop(t *testing.T) {
	// The Start and Stop tests are almost identical so they are merged into a single table.

	t.Parallel()

	type command bool

	const (
		start command = false
		stop  command = true
	)

	testCases := map[string]struct {
		dontRegisterDistro bool
		wslErr             bool
		cmd                command

		wantState wsl.State
		wantErr   bool
	}{
		"With command Start": {cmd: start, wantState: wsl.Running},
		"With command Stop":  {cmd: stop, wantState: wsl.Stopped},

		"Error with Start when the distro does not exist": {cmd: start, dontRegisterDistro: true, wantErr: true},
		"Error with Stop when the distro does not exist":  {cmd: stop, dontRegisterDistro: true, wantErr: true},

		"Error with Start when WSL returns error": {cmd: start, wslErr: true, wantErr: true},
		"Error with Stop when WSL returns error":  {cmd: stop, wslErr: true, wantErr: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			testReceiveCommand(t, distroSettings{install: !tc.dontRegisterDistro}, t.TempDir(), t.TempDir(),
				// Test setup
				func(testBed *commandTestBed) *landscapeapi.Command {
					if tc.wslErr {
						testBed.wslMock.WslLaunchInteractiveError = true
					}

					if tc.cmd == start {
						return &landscapeapi.Command{
							Cmd: &landscapeapi.Command_Start_{Start: &landscapeapi.Command_Start{Id: testBed.distro.Name()}},
						}
					}

					return &landscapeapi.Command{
						Cmd: &landscapeapi.Command_Stop_{Stop: &landscapeapi.Command_Stop{Id: testBed.distro.Name()}},
					}
				},
				// Test assertions
				func(testBed *commandTestBed) {
					const maxTimeout = 10 * time.Second
					const tickRate = time.Second

					if tc.wantErr {
						ok, _ := checkEventuallyState(t, testBed.distro, tc.wantState, maxTimeout, tickRate)
						require.False(t, ok, "State %q should never have been reached", tc.wantState)
						return
					}

					ok, state := checkEventuallyState(t, testBed.distro, tc.wantState, maxTimeout, tickRate)
					require.True(t, ok, "Distro never reached %q state. Last state: %q", tc.wantState, state)
				})
		})
	}
}

func TestInstall(t *testing.T) {
	const brokenURL = "@:?/BROKEN_URL"

	t.Parallel()

	testCases := map[string]struct {
		noCloudInit            bool
		cloudInitWriteErr      bool
		corruptDb              bool
		distroAlreadyInstalled bool
		distroName             string
		wslInstallErr          bool
		wslRegisterErr         bool
		wslLaunchErr           bool
		appxDoesNotExist       bool
		nonResponsiveServer    bool
		breakVhdxDir           bool
		breakTarFile           bool
		breakTempDir           bool
		cloudInitExecFailure   bool
		isTarBased             bool
		setDefaultUserErr      bool
		getDefaultUserErr      bool
		createdByLandscape     bool

		sendRootfsURL    string
		requestID        string
		missingChecksums bool

		wantCloudInitWriteCalled  bool
		wantCloudInitRemoveCalled bool
		wantInstalled             bool
	}{
		"From the store":                    {wantInstalled: true, wantCloudInitWriteCalled: true},
		"From a tar-based distro":           {isTarBased: true, wantInstalled: true, wantCloudInitWriteCalled: true},
		"From a rootfs URL with a checksum": {sendRootfsURL: "goodfile", wantInstalled: true},
		"With no cloud-init":                {noCloudInit: true, wantCloudInitWriteCalled: true, wantInstalled: true},
		"With no checksum file":             {missingChecksums: true, sendRootfsURL: "goodfile", wantInstalled: true},
		"With cloud-init failure":           {sendRootfsURL: "goodfile", cloudInitExecFailure: true, wantInstalled: true},

		"Error when the distroname is empty":          {distroName: "-"},
		"Error when the Appx does not exist":          {appxDoesNotExist: true},
		"Error when the distro is already installed":  {distroAlreadyInstalled: true, wantInstalled: true},
		"Error when the distro fails to install":      {wslInstallErr: true, wantCloudInitRemoveCalled: true},
		"Error when cannot write cloud-init file":     {cloudInitWriteErr: true, wantCloudInitWriteCalled: true},
		"Error when registration fails":               {isTarBased: true, wslRegisterErr: true, wantInstalled: false},
		"Error when the distro db is corrupted":       {isTarBased: true, corruptDb: true, wantInstalled: false},
		"Error when default user cannot be retrieved": {isTarBased: true, getDefaultUserErr: true, wantInstalled: false},
		"Error when default user cannot be set":       {isTarBased: true, setDefaultUserErr: true, wantInstalled: false},

		"Error when launching the new distro fails":                       {wslLaunchErr: true, sendRootfsURL: "goodfile", wantInstalled: false},
		"Error when the distro ID is reserved (Ubuntu)":                   {sendRootfsURL: "goodfile", distroName: "Ubuntu", wantInstalled: false},
		"Error when the distro ID is reserved (Preview)":                  {sendRootfsURL: "goodfile", distroName: "Ubuntu-Preview", wantInstalled: false},
		"Error when the distro ID is reserved (case sensitiveness)":       {sendRootfsURL: "goodfile", distroName: "ubuntu-preview", wantInstalled: false},
		"Error when the distro ID is reserved (release numbers)":          {sendRootfsURL: "goodfile", distroName: "Ubuntu-19.13", wantInstalled: false},
		"Error when the distro ID is reserved (release numbers and case)": {sendRootfsURL: "goodfile", distroName: "uBuntu-19.13", wantInstalled: false},
		"Error when the rootfs isn't a valid tarball":                     {sendRootfsURL: "badfile", wantInstalled: false},
		"Error when the checksum doesn't match":                           {sendRootfsURL: "badchecksum", wantInstalled: false},
		"Error when the checksum entry is missing for the rootfs":         {sendRootfsURL: "rootfswithnochecksum", wantInstalled: false},
		"Error when the rootfs doesn't exist":                             {sendRootfsURL: "badresponse", wantInstalled: false},
		"Error when the rootfs URL is ill-formed":                         {sendRootfsURL: brokenURL, wantInstalled: false},
		"Error when URL doesn't respond":                                  {sendRootfsURL: "goodfile", nonResponsiveServer: true, wantInstalled: false},
		"Error when the destination dir for the VHDX cannot be created":   {sendRootfsURL: "goodfile", breakVhdxDir: true, wantInstalled: false},
		"Error when the rootfs tarball cannot be created":                 {sendRootfsURL: "goodfile", breakTarFile: true, wantInstalled: false},
		"Error when the rootfs temporary dir cannot be created":           {sendRootfsURL: "goodfile", breakTempDir: true, wantInstalled: false},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			settings := distroSettings{
				name: testDistroAppx,
			}

			if tc.appxDoesNotExist || tc.sendRootfsURL != "" {
				switch tc.distroName {
				case "-":
					settings.name = ""
				case "":
					settings.name = wsltestutils.RandomDistroName(t)
				default:
					settings.name = tc.distroName
				}
			}

			if tc.isTarBased {
				settings.name = "Ubuntu-24.04"
			}

			if tc.distroAlreadyInstalled {
				settings.install = true
			}

			// Here we depend on implementation details to increase test coverage :see_no_evil:
			home := t.TempDir()
			if tc.breakVhdxDir {
				err := os.MkdirAll(filepath.Join(home, "WSL"), 0700)
				require.NoError(t, err, "Setup: creating destination dir shouldn't fail")
				f, err := os.Create(filepath.Join(home, "WSL", settings.name))
				require.NoError(t, err, "Setup: breaking the destination dir shouldn't fail")
				f.Close()
			}

			downloadDir := t.TempDir()
			if tc.breakTarFile {
				err := os.MkdirAll(filepath.Join(downloadDir, settings.name, settings.name+".tar.gz"), 0700)
				require.NoError(t, err, "Setup: breaking the destination tarball shouldn't fail")
			}

			if tc.breakTempDir {
				f, err := os.Create(filepath.Join(downloadDir, settings.name))
				require.NoError(t, err, "Setup: breaking the destination temp dir shouldn't fail")
				f.Close()
			}

			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			fileServerAddr := mockRootfsFileServer(t, ctx, !tc.missingChecksums)

			testReceiveCommand(t, settings, home, downloadDir,
				// Test setup
				func(testBed *commandTestBed) *landscapeapi.Command {
					var distroName string
					switch tc.distroName {
					case "-":
						distroName = ""
					case "":
						distroName = testBed.distro.Name()
					default:
						distroName = tc.distroName
					}
					if tc.cloudInitWriteErr {
						testBed.cloudInit.writeErr = true
					}

					if tc.wslInstallErr {
						testBed.wslMock.InstallError = true
					}

					if tc.wslRegisterErr {
						testBed.wslMock.WslRegisterDistributionError = true
					}

					if tc.wslLaunchErr {
						testBed.wslMock.WslLaunchInteractiveError = true
						testBed.wslMock.WslLaunchError = true
					}

					if tc.cloudInitExecFailure {
						testBed.wslMock.WslLaunchInteractiveError = true
					}

					if tc.getDefaultUserErr {
						testBed.wslMock.WslGetDistributionConfigurationError = true
					}

					if tc.setDefaultUserErr {
						testBed.wslMock.WslConfigureDistributionError = true
					}

					if tc.corruptDb {
						require.NoError(t, os.RemoveAll(filepath.Join(home, consts.DatabaseFileName)), "Setup: removing the database file should not fail")
						require.NoError(t, os.MkdirAll(filepath.Join(home, consts.DatabaseFileName), 0750), "Setup: breaking the database file should not fail")
					}

					var cloudInit string
					if !tc.noCloudInit {
						cloudInit = "Hello, this is a cloud-init file"
					}

					if tc.sendRootfsURL == "" {
						return &landscapeapi.Command{
							Cmd:       &landscapeapi.Command_Install_{Install: &landscapeapi.Command_Install{Id: distroName}},
							RequestId: tc.requestID,
						}
					}

					u := tc.sendRootfsURL
					var err error
					if tc.sendRootfsURL != brokenURL {
						u, err = url.JoinPath(fileServerAddr, "/releases/theone", tc.sendRootfsURL)
						require.NoError(t, err, "Setup: could not assemble URL: %s + %s", fileServerAddr, tc.sendRootfsURL)
					}

					if tc.nonResponsiveServer {
						u = "localhost:9"
					}

					return &landscapeapi.Command{
						Cmd: &landscapeapi.Command_Install_{Install: &landscapeapi.Command_Install{
							Id:        distroName,
							Cloudinit: &cloudInit,
							RootfsURL: &u,
						}},
						RequestId: tc.requestID,
					}
				},
				// Test assertions
				func(testBed *commandTestBed) {
					const timeout = 10 * time.Second // Installation can take a while

					if tc.wantInstalled {
						require.Eventually(t, func() bool {
							registered, err := testBed.distro.IsRegistered()
							if err != nil {
								return false
							}
							return registered
						}, timeout, 100*time.Millisecond, "Distro should have been registered")

						if tc.createdByLandscape {
							dbDistro, _ := testBed.db.Get(testBed.distro.Name())
							require.True(t, dbDistro.Properties().CreatedByLandscape, "CreatedByLandscape should be true!")
						}
					} else {
						time.Sleep(timeout)

						distroExists, err := testBed.distro.IsRegistered()
						require.NoError(t, err, "IsRegistered should return no error")
						require.False(t, distroExists, "Distro should not have been registered")
					}

					if tc.wantCloudInitWriteCalled {
						require.True(t, testBed.cloudInit.writeCalled.Load(), "Cloud-init should have been called to write the user data file")
					}

					if tc.requestID != "" {
						require.True(t, testBed.cloudInit.instanceIDSet.Load(), "Cloud-init should have set the metadata instance ID")
					}

					if tc.wantCloudInitRemoveCalled {
						require.True(t, testBed.cloudInit.removeCalled.Load(), "Cloud-init should have been called to remove the user data file")
					}
				})
		})
	}
}

type cmdType int

const (
	assignHost cmdType = iota
	install
	start
	stop
	uninstall
	setDefault
	shutdownHost
	unknown
)

func TestSendStatusComplete(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		requestID               string
		preinstallDistro        bool
		cmdType                 any
		serverErrorOnSendStatus bool

		wantNoCommandStatus bool
		wantErr             bool
	}{
		"AssignHost sends no status message":   {cmdType: assignHost, wantNoCommandStatus: true},
		"Success starting an installed distro": {cmdType: start, requestID: "123abc", preinstallDistro: true},
		"Success while failing to send status": {cmdType: uninstall, requestID: "123abc", preinstallDistro: true, serverErrorOnSendStatus: true},
		"Success setting default distro":       {cmdType: setDefault, requestID: "456def", preinstallDistro: true},

		"Complete Start command with error when distro is not installed":     {cmdType: start, requestID: "start_err", wantErr: true},
		"Complete Stop command with error when distro is not installed":      {cmdType: stop, requestID: "stop_err", wantErr: true},
		"Complete Install command with error when download fails":            {cmdType: install, requestID: "install_err", wantErr: true},
		"Complete Uninstall command with error when distro is not installed": {cmdType: uninstall, requestID: "uninstall_err", wantErr: true},
		"Complete Unknown command with error":                                {cmdType: unknown, requestID: "unknown_err", wantErr: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			testReceiveCommand(t, distroSettings{install: tc.preinstallDistro}, t.TempDir(), t.TempDir(),
				// Test setup
				func(testBed *commandTestBed) *landscapeapi.Command {
					if tc.serverErrorOnSendStatus {
						testBed.serverService.SendCmdStatusError = errors.New("Mock error")
					}

					switch tc.cmdType {
					case assignHost:
						return &landscapeapi.Command{
							Cmd: &landscapeapi.Command_AssignHost_{AssignHost: &landscapeapi.Command_AssignHost{Uid: "HostUID123"}},
						}
					case install:
						u := "localhost:9"
						return &landscapeapi.Command{
							Cmd: &landscapeapi.Command_Install_{Install: &landscapeapi.Command_Install{
								Id:        "distroName",
								RootfsURL: &u,
							}},
							RequestId: tc.requestID,
						}
					case start:
						return &landscapeapi.Command{
							Cmd:       &landscapeapi.Command_Start_{Start: &landscapeapi.Command_Start{Id: testBed.distro.Name()}},
							RequestId: tc.requestID,
						}
					case stop:
						return &landscapeapi.Command{
							Cmd:       &landscapeapi.Command_Stop_{Stop: &landscapeapi.Command_Stop{Id: testBed.distro.Name()}},
							RequestId: tc.requestID,
						}
					case uninstall:
						return &landscapeapi.Command{
							Cmd:       &landscapeapi.Command_Uninstall_{Uninstall: &landscapeapi.Command_Uninstall{Id: testBed.distro.Name()}},
							RequestId: tc.requestID,
						}
					case setDefault:
						return &landscapeapi.Command{
							Cmd:       &landscapeapi.Command_SetDefault_{SetDefault: &landscapeapi.Command_SetDefault{Id: testBed.distro.Name()}},
							RequestId: tc.requestID,
						}
					default:
						return &landscapeapi.Command{RequestId: tc.requestID}
					}
				},
				// Test assertions
				func(testBed *commandTestBed) {
					const maxTimeout = 5 * time.Second
					const tickRate = 100 * time.Millisecond

					// Add some wait time to ensure all status messages were received by the mock server.
					timer := time.NewTimer(maxTimeout)
					defer timer.Stop()

					ticker := time.NewTicker(tickRate)
					defer ticker.Stop()

					var statusLogs []landscapemockservice.CmdStatusMsg
					for {
						select {
						case <-timer.C:
							if tc.wantNoCommandStatus {
								require.Empty(t, statusLogs, "No CommandStatus messages should have been sent")
								return
							}

							// Failure path: let's log the status messages to help debugging.
							t.Logf("CommandStatus log: %v", statusLogs)
							wantErr := "no"
							if tc.wantErr {
								wantErr = "an"
							}
							require.Failf(t, "CommandStatus log does not contain a matching message", "Request ID %q with state Complete and %s error.", tc.requestID, wantErr)
						case <-ticker.C:
							// Tick: let's see if the message we're looking for arrived.
							statusLogs = testBed.serverService.CommandStatusLog()

							if slices.ContainsFunc(statusLogs, func(msg landscapemockservice.CmdStatusMsg) bool {
								hasError := msg.Error != ""
								return msg.RequestID == tc.requestID && hasError == tc.wantErr && msg.CommandState == landscapeapi.CommandState_Completed
							}) {
								return
							} // else try again at next tick.
						}
					}
				})
		})
	}
}

//nolint:revive // Context goes after testing.T
func mockRootfsFileServer(t *testing.T, ctx context.Context, enableChecksumsFile bool) string {
	t.Helper()

	mux := http.NewServeMux()

	const getFile = "GET /releases/theone/%s"

	mux.HandleFunc(fmt.Sprintf(getFile, "goodfile"), func(w http.ResponseWriter, r *http.Request) {})             // Return empty file
	mux.HandleFunc(fmt.Sprintf(getFile, "badchecksum"), func(w http.ResponseWriter, r *http.Request) {})          // Return empty file
	mux.HandleFunc(fmt.Sprintf(getFile, "rootfswithnochecksum"), func(w http.ResponseWriter, r *http.Request) {}) // intentionally not in the checksums file
	mux.HandleFunc(fmt.Sprintf(getFile, "badfile"), func(w http.ResponseWriter, r *http.Request) {
		_, err := fmt.Fprintf(w, "MOCK_ERROR")
		if err != nil {
			t.Logf("mockRootfsFileServer: could not write response: %v", err)
		}
	})
	mux.HandleFunc(fmt.Sprintf(getFile, "badresponse"), func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	if enableChecksumsFile {
		mux.HandleFunc(fmt.Sprintf(getFile, "SHA256SUMS"), func(w http.ResponseWriter, r *http.Request) {
			_, err := fmt.Fprintf(w, `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 *goodfile
		afe55cda4210c2439b47c62c01039027522f7ed4abdb113972b3030b3359532a *badfile
		1234 *badchecksum
		5678 *badresponse badresponse
		5678 *badresponse`,
			)
			if err != nil {
				t.Logf("mockRootfsFileServer: could not write response: %v", err)
			}
		})
	}

	lis, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "localhost:")
	require.NoError(t, err, "Setup: mockRootfsFileServer could not listen")

	go func() {
		s := &http.Server{
			Handler:      mux,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 5 * time.Second,
		}
		if err := s.Serve(lis); err != nil {
			t.Logf("mockRootfsFileServer: serve error: %v", err)
		}
	}()

	t.Cleanup(func() {
		if err := lis.Close(); err != nil {
			t.Logf("Cleanup: could not close mock fileserver: %v", err)
		}
	})

	addr := "http://" + lis.Addr().String()
	t.Logf("Serving on %s", addr)
	return addr
}

func TestUninstall(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		distroNotInstalled bool
		wslUninstallErr    bool
		cloudInitRemoveErr bool

		wantNotRegistered         bool
		wantCloudInitRemoveCalled bool
	}{
		"Success": {wantNotRegistered: true, wantCloudInitRemoveCalled: true},

		"Error when the distroname does not match any distro":     {distroNotInstalled: true, wantNotRegistered: true},
		"Error when the distro fails to uninstall":                {wslUninstallErr: true},
		"Error when cloud-init cannot remove the cloud-init file": {cloudInitRemoveErr: true, wantNotRegistered: true, wantCloudInitRemoveCalled: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			testReceiveCommand(t, distroSettings{install: !tc.distroNotInstalled}, t.TempDir(), t.TempDir(),
				// Test setup
				func(testBed *commandTestBed) *landscapeapi.Command {
					if tc.cloudInitRemoveErr {
						testBed.cloudInit.removeErr = true
					}

					if tc.wslUninstallErr {
						testBed.wslMock.WslUnregisterDistributionError = true
					}

					return &landscapeapi.Command{
						Cmd: &landscapeapi.Command_Uninstall_{Uninstall: &landscapeapi.Command_Uninstall{Id: testBed.distro.Name()}},
					}
				},
				// Test assertions
				func(testBed *commandTestBed) {
					const maxTimeout = 20 * time.Second // Uninstalling can take a while

					if tc.wantNotRegistered {
						ok, _ := checkEventuallyState(t, testBed.distro, wsl.NonRegistered, maxTimeout, time.Second)
						require.True(t, ok, "Distro should not be registered")
					} else {
						time.Sleep(maxTimeout)
						distroExists, err := testBed.distro.IsRegistered()
						require.NoError(t, err, "IsRegistered should return no error")
						require.True(t, distroExists, "Existing distro should still have been unregistered")
					}

					if tc.wantCloudInitRemoveCalled {
						require.Eventually(t, testBed.cloudInit.removeCalled.Load, time.Second, 100*time.Millisecond,
							"Cloud-init should have been called to write the user data file")
					}
				})
		})
	}
}

func TestSetDefaultDistro(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		distroNotInstalled bool
		wslSetDefaultErr   bool
		alreadyDefault     bool

		wantSetAsDefault bool
	}{
		"Success":                             {wantSetAsDefault: true},
		"Success when it was already default": {alreadyDefault: true, wantSetAsDefault: true},

		"Error when the distro name does not match an existing distro": {distroNotInstalled: true},
		"Error when WSL SetDefault fails":                              {wslSetDefaultErr: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			testReceiveCommand(t, distroSettings{install: !tc.distroNotInstalled}, t.TempDir(), t.TempDir(),
				// Test setup
				func(testBed *commandTestBed) *landscapeapi.Command {
					if !tc.alreadyDefault {
						name, _ := wsltestutils.RegisterDistro(t, testBed.ctx, false)
						d := wsl.NewDistro(testBed.ctx, name)
						err := d.SetAsDefault()
						require.NoError(t, err, "Setup: could not set another distro as default")
					}

					if tc.wslSetDefaultErr {
						testBed.wslMock.SetAsDefaultError = true
					}

					return &landscapeapi.Command{
						Cmd: &landscapeapi.Command_SetDefault_{SetDefault: &landscapeapi.Command_SetDefault{Id: testBed.distro.Name()}},
					}
				},
				// Test assertions
				func(testBed *commandTestBed) {
					const maxTimeout = 20 * time.Second // Uninstalling can take a while

					if tc.wantSetAsDefault {
						require.Eventually(t, func() bool {
							d, ok, err := wsl.DefaultDistro(testBed.ctx)
							if err != nil {
								return false
							}
							if !ok {
								return false
							}
							return d.Name() == testBed.distro.Name()
						}, maxTimeout, time.Second, "Distro should have been made default")
					} else {
						time.Sleep(maxTimeout)
						d, ok, err := wsl.DefaultDistro(testBed.ctx)
						require.NoError(t, err, "DefaultDistro should return no error")
						require.True(t, ok, "There should be a default distro")
						require.NotEqual(t, testBed.distro.Name(), d.Name(), "Distro should not have been default")
					}
				})
		})
	}
}

func TestSetShutdownHost(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		wslShutdownErr bool

		wantStopped bool
	}{
		"Success": {wantStopped: true},

		"Error when the WSL Shutdown fails": {wslShutdownErr: true},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			testReceiveCommand(t, distroSettings{install: true}, t.TempDir(), t.TempDir(),
				// Test setup
				func(testBed *commandTestBed) *landscapeapi.Command {
					d := wsl.NewDistro(testBed.ctx, testBed.distro.Name())
					err := d.Command(testBed.ctx, "exit 0").Run()
					require.NoError(t, err, "Setup: could not start distro")

					if tc.wslShutdownErr {
						testBed.wslMock.ShutdownError = true
					}

					return &landscapeapi.Command{
						Cmd: &landscapeapi.Command_ShutdownHost_{ShutdownHost: &landscapeapi.Command_ShutdownHost{}},
					}
				},
				// Test assertions
				func(testBed *commandTestBed) {
					const maxTimeout = 5 * time.Second

					if !tc.wantStopped {
						time.Sleep(maxTimeout)
						state := wsltestutils.DistroState(t, testBed.ctx, testBed.distro.Name())
						require.Equal(t, "Running", state, "Distro should not have stopped")
						return
					}

					require.Eventually(t, func() bool {
						return wsltestutils.DistroState(t, testBed.ctx, testBed.distro.Name()) == "Stopped"
					}, maxTimeout, time.Second, "Distro should have stopped")
				})
		})
	}
}

// commandTestBed is a bag of data with all the necessary utils to run executor tests.
type commandTestBed struct {
	ctx context.Context

	conf      *mockConfig
	distro    *wsl.Distro
	db        *database.DistroDB
	cloudInit *mockCloudInit

	serverService *landscapemockservice.Service
	clientService *landscape.Service

	wslMock *wslmock.Backend
}

// distroSettings tells testReceiveCommand what the test distro should be like.
type distroSettings struct {
	install bool

	// set name to empty to auto-generate one
	name string
}

// testReceiveCommand contains all the boilerplate necessary to test the Landscape executor.
//
// Before testSetup:
//   - Set up the mock WSL
//   - Set up the agent components (config, database, cloud-init...)
//   - Set up the mock Landscape server
//   - Set up the landscape client
//   - Register a distro to test
//
// Then, testSetup is called. After this:
//   - Send the command
//
// Then, testAssertions is called.
func testReceiveCommand(t *testing.T, distrosettings distroSettings, homedir string, downloaddir string, testSetup func(*commandTestBed) *landscapeapi.Command, testAssertions func(*commandTestBed)) {
	t.Helper()
	var tb commandTestBed

	if !wsl.MockAvailable() {
		t.Skip("This test can only run with the mock")
	}

	// Set up WSL mock
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	ctx = context.WithValue(ctx, landscape.InsecureCredentials, true)

	tb.wslMock = wslmock.New()
	ctx = wsl.WithMock(ctx, tb.wslMock)

	tb.ctx = ctx

	// Set up Landscape server
	lis, server, service := setUpLandscapeMock(t, ctx, "localhost:", "")
	context.AfterFunc(ctx, func() { _ = lis.Close() })

	tb.serverService = service

	//nolint: errcheck // We know it is going to exit with "context cancelled"
	go server.Serve(lis)
	context.AfterFunc(ctx, func() { server.Stop() })

	// Set up agent components (config, database, etc.)
	if tb.conf == nil {
		tb.conf = &mockConfig{
			proToken:              "TOKEN",
			landscapeClientConfig: executeLandscapeConfigTemplate(t, defaultLandscapeConfig, "", lis.Addr()),
		}
	}

	if homedir == "" {
		homedir = t.TempDir()
	}
	db, err := database.New(ctx, homedir)
	require.NoError(t, err, "Setup: database New should not return an error")

	tb.db = db
	tb.cloudInit = &mockCloudInit{}

	// Set up Landscape client
	if downloaddir == "" {
		downloaddir = t.TempDir()
	}
	clientService, err := landscape.New(ctx, tb.conf, tb.db, tb.cloudInit, landscape.WithHostname("HOSTNAME"), landscape.WithHomeDir(homedir), landscape.WithDownloadDir(downloaddir))
	require.NoError(t, err, "Landscape NewClient should not return an error")

	err = clientService.Connect()
	require.NoError(t, err, "Setup: Connect should return no errors")

	tb.clientService = clientService
	t.Cleanup(func() { tb.clientService.Stop(ctx) })

	require.Eventually(t, func() bool {
		return clientService.Connected() && tb.conf.landscapeAgentUID != "" && service.IsConnected(tb.conf.landscapeAgentUID)
	}, 10*time.Second, 100*time.Millisecond, "Setup: Landscape server and client never made a connection")

	// Set up test distro
	//
	// This must be done AFTER having set up the Landscape client and server. When these two connect for the first time,
	// the server sends the client a UID. This UID is the distributed to all distros, waking them up. This interferes with
	// the tests for Start and Stop as we cannot really assert what started the distro.
	//
	// Hence, we register the distro after the client and server have connected. In production, this would still wake up the
	// distros but our tests mock the Config so that ProvisioningTasks always returns an empty list.
	if distrosettings.name == "" {
		distrosettings.name = wsltestutils.RandomDistroName(t)
	}

	if distrosettings.install {
		d := wsl.NewDistro(ctx, distrosettings.name)
		tb.distro = &d

		err = d.Register(fakeRootFS(t))
		require.NoError(t, err) // Error messsage is explanatory enough

		dbDistro, err := db.GetDistroAndUpdateProperties(ctx, d.Name(), distro.Properties{
			CreatedByLandscape: true,
		})
		require.NoError(t, err, "Setup: GetDistroAndUpdateProperties should return no errors")
		context.AfterFunc(ctx, func() { dbDistro.Cleanup(ctx) })
	} else {
		d := wsl.NewDistro(ctx, distrosettings.name)
		tb.distro = &d
	}

	// Exectute test setup
	command := testSetup(&tb)

	// Send (and receive command)
	err = tb.serverService.SendCommand(ctx, tb.conf.landscapeAgentUID, command)
	require.NoError(t, err, "Setup: SendCommand should return no error")

	// Execute test assertions
	testAssertions(&tb)
}

func fakeRootFS(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	rootfs := filepath.Join(dir, "empty.tar.gz")
	err := os.WriteFile(rootfs, []byte{}, 0600)
	require.NoError(t, err, "Setup: could not write empty fake rootfs")

	return rootfs
}

func checkEventuallyState(t *testing.T, d interface{ State() (wsl.State, error) }, wantState wsl.State, waitFor, tick time.Duration) (ok bool, lastState wsl.State) {
	t.Helper()

	timer := time.NewTimer(waitFor)
	defer timer.Stop()

	ticker := time.NewTicker(tick)
	defer ticker.Stop()

	for {
		select {
		case <-timer.C:
			return false, lastState
		case <-ticker.C:
			var err error
			lastState, err = d.State()
			require.NoError(t, err, "disto State should return no error")
			if lastState == wantState {
				return true, lastState
			}
		}
	}
}
