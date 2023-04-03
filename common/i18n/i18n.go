// Package i18n is responsible for internationalization/translation handling and generation.
package i18n

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/snapcore/go-gettext"
)

type i18n struct {
	domain    string
	localeDir string
	loc       string

	gettext.Catalog
	translations gettext.TextDomain
}

var (
	locale i18n

	// G is the shorthand for Gettext.
	G = func(msgid string) string { return msgid }
	// NG is the shorthand for NGettext.
	NG = func(msgid string, msgidPlural string, n uint32) string { return msgid }
)

type Option func(l *i18n)

// InitI18nDomain calls bind + set locale to system values.
func InitI18nDomain(domain string, args ...Option) {
	locale = i18n{
		domain:    domain,
		localeDir: "/usr/share/locale",
	}

	for _, f := range args {
		f(&locale)
	}

	locale.bindTextDomain(locale.domain, locale.localeDir)
	locale.setLocale(locale.loc)

	G = locale.Gettext
	NG = locale.NGettext
}

// langpackResolver tries to fetch locale mo file path.
// It first checks for the real locale (e.g. de_DE) and then
// tries to simplify the locale (e.g. de_DE -> de).
func langpackResolver(root string, locale string, domain string) string {
	for _, locale := range []string{locale, strings.SplitN(locale, "_", 2)[0]} {
		r := filepath.Join(locale, "LC_MESSAGES", fmt.Sprintf("%s.mo", domain))

		// look into the generated mo files path first for translations, then the system
		var candidateDirs []string
		// Ubuntu uses /usr/share/locale-langpack and patches the glibc gettext implementation
		candidateDirs = append(candidateDirs, filepath.Join(root, "..", "locale-langpack"))
		candidateDirs = append(candidateDirs, root)

		for _, dir := range candidateDirs {
			candidateMo := filepath.Join(dir, r)
			// Only load valid candidates, if we can't access it or have perm issues, ignore
			if _, err := os.Stat(candidateMo); err != nil {
				continue
			}
			return candidateMo
		}
	}

	return ""
}

func (l *i18n) bindTextDomain(domain, dir string) {
	l.translations = gettext.TextDomain{
		Name:         domain,
		LocaleDir:    dir,
		PathResolver: langpackResolver,
	}
}

// setLocale initializes the locale name and simplify it.
// If empty, it defaults to system ones set in LC_MESSAGES and LANG.
func (l *i18n) setLocale(loc string) {
	if loc == "" {
		loc = os.Getenv("LC_MESSAGES")
		if loc == "" {
			loc = os.Getenv("LANG")
		}
	}
	// de_DE.UTF-8, de_DE@euro all need to get simplified
	loc = strings.Split(loc, "@")[0]
	loc = strings.Split(loc, ".")[0]

	l.Catalog = l.translations.Locale(loc)
}
