package system

import "testing"

const LandscapeConfigPath = landscapeConfigPath

func (s *System) CmdExeCache() *string {
	return &s.cmdExe
}

type RealBackend = realBackend
type StrictUTF16Transformer = strictUTF16Transformer

// ErrOddByteCount is the error returned by the strictUTF16Tranformer when the source contains an
// odd number of bytes, exported just for testing, as it's only an implementation detail.
var ErrOddByteCount = errOddByteCount

// ErrInvalidSurrogatePair is the error returned by the strictUTF16Transformer when a supposed
// surrogate pair doesn't meet the expected byte range, exported just for testing as it's only an
// implementation detail.
var ErrInvalidSurrogatePair = errInvalidSurrogatePair

// SetProAttachTmpDirForTest overrides the directory used by ProAttach for creating the temporary attach config file.
func SetProAttachTmpDirForTest(t *testing.T, dir string) {
	t.Helper()
	orig := proAttachTmpDir
	proAttachTmpDir = dir
	t.Cleanup(func() {
		proAttachTmpDir = orig
	})
}
