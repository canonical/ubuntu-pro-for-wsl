package controlstream_test

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"testing"
	"time"

	agentapi "github.com/canonical/ubuntu-pro-for-windows/agentapi/go"
	"github.com/canonical/ubuntu-pro-for-windows/wsl-pro-service/internal/controlstream"
	"github.com/canonical/ubuntu-pro-for-windows/wsl-pro-service/internal/testutils"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestMain(m *testing.M) {
	log.SetLevel(log.DebugLevel)

	m.Run()
}

func TestConnect(t *testing.T) {
	t.Parallel()

	type dataFileState int

	const (
		dataFileGood dataFileState = iota
		dataFileUnreadable
		dataFileNotExist
		dataFileEmpty
		dataFileBadSyntax
		dataFileBadData
	)

	testCases := map[string]struct {
		portFile              dataFileState
		breakWindowsLocalhost bool

		agentDoesntRecv   bool
		agentSendsNoPort  bool
		agentSendsBadPort bool

		wantErr bool
	}{
		"Success": {},

		// Port file errors
		"No connection because port file does not exist":             {portFile: dataFileNotExist, wantErr: true},
		"No connection because of unreadable port file":              {portFile: dataFileUnreadable, wantErr: true},
		"No connection because of empty port file":                   {portFile: dataFileEmpty, wantErr: true},
		"No connection because of port file with invalid contents":   {portFile: dataFileBadSyntax, wantErr: true},
		"No connection because of port file contains the wrong port": {portFile: dataFileBadData, wantErr: true},

		// Network errors
		"Error because WindowsForwardedLocalhost returns error": {breakWindowsLocalhost: true, wantErr: true},

		// Agent errors
		"Incomplete handshake because Agent never receives":     {agentDoesntRecv: true, wantErr: true},
		"Incomplete handshake because Agent never sends a port": {agentSendsNoPort: true, wantErr: true},
		"Incomplete handshake because Agent sends port :0":      {agentSendsBadPort: true, wantErr: true},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			system, mock := testutils.MockSystem(t)

			if tc.breakWindowsLocalhost {
				mock.SetControlArg(testutils.WslInfoErr)
			}

			var agentArgs []testutils.AgentOption
			if tc.agentDoesntRecv {
				agentArgs = append(agentArgs, testutils.WithDropStreamBeforeReceivingInfo())
			} else if tc.agentSendsNoPort {
				agentArgs = append(agentArgs, testutils.WithDropStreamBeforeSendingPort())
			} else if tc.agentSendsBadPort {
				agentArgs = append(agentArgs, testutils.WithSendBadPort())
			}

			portFile := mock.DefaultAddrFile()
			_, agentMetaData := testutils.MockWindowsAgent(t, ctx, portFile, agentArgs...)

			switch tc.portFile {
			case dataFileGood:
			case dataFileNotExist:
				err := os.Remove(portFile)
				require.NoError(t, err, "Setup: could not remove port file")
			case dataFileUnreadable:
				err := os.Remove(portFile)
				require.NoError(t, err, "Setup: could not remove port file")
				err = os.Mkdir(portFile, 0600)
				require.NoError(t, err, "Setup: could not create directory where port file should be")
			case dataFileEmpty:
				f, err := os.Create(portFile)
				require.NoError(t, err, "Setup: failed to create empty port file")
				f.Close()
			case dataFileBadSyntax:
				err := os.WriteFile(portFile, []byte("This text is not a valid IP address"), 0600)
				require.NoError(t, err, "Setup: failed to create port file with invalid contents")
			case dataFileBadData:
				lis, err := net.Listen("tcp4", "localhost:")
				require.NoError(t, err, "Setup: could not reserve an IP address to mess with port file")
				wrongAddr := lis.Addr().String()

				err = os.WriteFile(portFile, []byte(wrongAddr), 0600)
				require.NoError(t, err, "Setup: failed to create port file with misleading contents")

				err = lis.Close()
				require.NoError(t, err, "Setup: failed to close port file used to select wrong port")
			default:
				require.Fail(t, "Test setup error", "Unexpected enum value %d for portFile state", tc.portFile)
			}

			cs := controlstream.New(portFile, system)

			select {
			case <-cs.Done(ctx):
			case <-time.After(time.Second):
				require.Fail(t, "Done should not block before the control stream is connected")
			}

			err := cs.Connect(ctx)
			if tc.wantErr {
				require.Error(t, err, "Connect should have returned an error")
				return
			}
			require.NoError(t, err, "Connect should have returned no error")
			defer cs.Disconnect()

			require.Equal(t, int32(1), agentMetaData.ConnectionCount.Load(), "The agent should have received one connection")
			require.Equal(t, agentMetaData.ReservedPort.Load(), cs.ReservedPort(), "The Windows agent and the Daemon should agree on the reserved port")

			select {
			case <-cs.Done(ctx):
				require.Fail(t, "Done should not return while the control stream is connected")
			case <-time.After(time.Second):
			}

			cs.Disconnect()

			select {
			case <-cs.Done(ctx):
			case <-time.After(time.Second):
				require.Fail(t, "Done should not block after the control stream is disconnected")
			}

			// Ensure no panics
			cs.Disconnect()
		})
	}
}

func TestWithProMock(t *testing.T)     { testutils.ProMock(t) }
func TestWithWslPathMock(t *testing.T) { testutils.WslPathMock(t) }
func TestWithWslInfoMock(t *testing.T) { testutils.WslInfoMock(t) }
func TestWithCmdExeMock(t *testing.T)  { testutils.CmdExeMock(t) }
