package config

// Source indicates the method a configuration parameter was acquired.
type Source int

// config Source types.
const (
	// SourceNone -> no data.
	SourceNone Source = iota

	// SourceRegistry -> the data was obtained from the registry.
	SourceRegistry

	// SourceGUI -> the data was obtained by the GUI.
	SourceGUI

	// SourceMicrosoftStore -> the data was acquired via the Microsoft Store.
	SourceMicrosoftStore
)

type subscription struct {
	GUI          string
	Store        string
	Organization string `yaml:"-"`
	Checksum     string
}

func (s subscription) resolve() (string, Source) {
	if s.Store != "" {
		return s.Store, SourceMicrosoftStore
	}

	if s.GUI != "" {
		return s.GUI, SourceGUI
	}

	if s.Organization != "" {
		return s.Organization, SourceRegistry
	}

	return "", SourceNone
}

type landscapeConf struct {
	GUIConfig string `yaml:"config"`
	OrgConfig string `yaml:"-"`

	UID      string
	Checksum string
}

func (p landscapeConf) resolve() (string, Source) {
	if p.GUIConfig != "" {
		return p.GUIConfig, SourceGUI
	}

	if p.OrgConfig != "" {
		return p.OrgConfig, SourceRegistry
	}

	return "", SourceNone
}
