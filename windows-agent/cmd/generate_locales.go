//go:build tools

package main

import (
	"os"

	"github.com/canonical/ubuntu-pro-for-windows/common"
	"github.com/canonical/ubuntu-pro-for-windows/tools/locales"
)

func main() {
	config := locales.Configuration{
		Domain:    common.TEXTDOMAIN,
		PotFile:   "../po/ubuntu-pro.pot",
		LocaleDir: "../po/",
		MoDir:     "../../generated/windows-agent/",
		RootDir:   "..",
	}

	verb := "help"
	var args []string

	if len(os.Args) > 1 {
		verb = os.Args[1]
		args = os.Args[2:]
	}

	locales.Generate(verb, config, args...)
}
