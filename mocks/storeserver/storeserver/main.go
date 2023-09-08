// Package main runs the MS Store server mock as its own process.
package main

import (
	"os"

	"github.com/canonical/ubuntu-pro-for-windows/mocks/restserver"
	"github.com/canonical/ubuntu-pro-for-windows/mocks/storeserver/storemockserver"
)

func serverFactory(settings restserver.Settings) restserver.Server {
	innerSettings, ok := settings.(*storemockserver.Settings)
	if !ok {
		panic("Cannot receive my own settings")
	}
	return storemockserver.NewServer(*innerSettings)
}

func main() {
	defaultSettings := storemockserver.DefaultSettings()

	app := restserver.Application{
		Name:            "Store Server",
		Description:     "MS Store API",
		DefaultSettings: &defaultSettings,
		ServerFactory:   serverFactory,
	}

	os.Exit(app.Execute())
}
