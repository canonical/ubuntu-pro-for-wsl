// Package main runs the contract server mock as its own process.
package main

import (
	"os"

	"github.com/canonical/ubuntu-pro-for-windows/mocks/contractserver/contractsmockserver"
	"github.com/canonical/ubuntu-pro-for-windows/mocks/restserver"
)

func serverFactory(settings restserver.Settings) restserver.Server {
	innerSettings, ok := settings.(*contractsmockserver.Settings)
	if !ok {
		panic("Cannot receive my own settings")
	}
	return contractsmockserver.NewServer(*innerSettings)
}

func main() {
	defaultSettings := contractsmockserver.DefaultSettings()

	app := restserver.Application{
		Name:            "contract server",
		Description:     "contract server",
		DefaultSettings: &defaultSettings,
		ServerFactory:   serverFactory,
	}

	os.Exit(app.Execute())
}
