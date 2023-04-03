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
		MoDir:     "../../generated/wsl-pro-service/",
		RootDir:   "..",
	}

	verb := "help"
	if len(os.Args) > 1 {
		verb = os.Args[1]
	}

	locales.Generate(verb, config)
}
