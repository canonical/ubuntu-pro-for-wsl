// Package i18n is responsible for internationalization/translation handling and generation.
package i18n

var (
	// G is the shorthand for Gettext.
	G = func(msgid string) string { return msgid }
	// NG is the shorthand for NGettext.
	NG = func(msgid string, msgidPlural string, n uint32) string { return msgid }
)
