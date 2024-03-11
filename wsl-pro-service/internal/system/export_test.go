package system

const LandscapeConfigPath = landscapeConfigPath

func (s *System) CmdExeCache() *string {
	return &s.cmdExe
}

type RealBackend = realBackend
