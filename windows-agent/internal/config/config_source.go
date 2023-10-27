package config

import "fmt"

// Source indicates the method a configuration parameter was acquired.
type Source int

// config Source types.
const (
	// SourceNone -> no data.
	SourceNone Source = iota

	// SourceRegistry -> the data was obtained from the registry.
	SourceRegistry

	// SourceUser -> the data was introduced by the user.
	SourceUser

	// SourceMicrosoftStore -> the data was acquired via the Microsoft Store.
	SourceMicrosoftStore
)

type subscription struct {
	User         string
	Store        string
	Organization string `yaml:"-"`
	Checksum     string
}

func (s subscription) resolve() (string, Source) {
	if s.Store != "" {
		return s.Store, SourceMicrosoftStore
	}

	if s.User != "" {
		return s.User, SourceUser
	}

	if s.Organization != "" {
		return s.Organization, SourceRegistry
	}

	return "", SourceNone
}

func (s *subscription) Set(src Source, proToken string) {
	ptr := s.src(src)
	*ptr = proToken
}

func (s subscription) Get(src Source) string {
	return *s.src(src)
}

// src is a helper to avoid duplicating the mapping in Get and Set.
func (s *subscription) src(src Source) *string {
	switch src {
	case SourceNone:
		// TODO: Panic? Warning?
		return new(string)
	case SourceUser:
		return &s.User
	case SourceRegistry:
		return &s.Organization
	case SourceMicrosoftStore:
		return &s.Store
	}

	panic(fmt.Sprintf("Unknown enum value for SubscriptionSource: %d", src))
}

type landscapeConf struct {
	UserConfig string `yaml:"config"`
	OrgConfig  string `yaml:"-"`

	UID      string
	Checksum string
}

func (p landscapeConf) resolve() (string, Source) {
	if p.UserConfig != "" {
		return p.UserConfig, SourceUser
	}

	if p.OrgConfig != "" {
		return p.OrgConfig, SourceRegistry
	}

	return "", SourceNone
}
