// Package main runs the MS Store server mock as its own process.
package main

import (
	"os"

	"github.com/canonical/ubuntu-pro-for-windows/mocks/restserver"
	"github.com/canonical/ubuntu-pro-for-windows/mocks/storeserver/storemockserver"
)

func serverFactory(settings restserver.Settings) restserver.Server {
	//nolint:forcetypeassert // Let the type coersion panic on failure.
	return storemockserver.NewServer(settings.(storemockserver.Settings))
}

func main() {
	app := restserver.App{
		Name:            "Store Server",
		Description:     "MS Store API",
		DefaultSettings: storemockserver.DefaultSettings(),
		ServerFactory:   serverFactory,
	}

	os.Exit(app.Run())
}
