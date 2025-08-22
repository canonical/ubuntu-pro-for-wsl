package ui

// LandscapeListener is the channel via which tests can read Landscape connection events.
func (s *Service) LandscapeListener() chan error {
	return s.landscapeListener
}
