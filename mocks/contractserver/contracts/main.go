// Package main runs the contract server mock as its own process.
package main

import (
	"os"

	"github.com/canonical/ubuntu-pro-for-windows/mocks/contractserver/contractsmockserver"
	"github.com/canonical/ubuntu-pro-for-windows/mocks/restserver"
)

func serverFactory(settings restserver.Settings) restserver.Server {
	//nolint:forcetypeassert // Let the type coersion panic on failure.
	return contractsmockserver.NewServer(settings.(contractsmockserver.Settings))
}

func main() {
	app := restserver.App{
		Name:            "contract server",
		Description:     "contract server",
		DefaultSettings: contractsmockserver.DefaultSettings(),
		ServerFactory:   serverFactory,
	}

	os.Exit(app.Run())
}
