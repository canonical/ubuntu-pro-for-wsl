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
