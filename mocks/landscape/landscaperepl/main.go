// package main contains a Landscape mock REPL
// execute the program and type "help" for usage information
package main

import (
	"fmt"
	"log/slog"
	"os"
)

func main() {
	app := New()
	os.Exit(run(app))
}

func run(a *App) int {
	if err := a.Run(); err != nil {
		slog.Error(fmt.Sprintf("Error: %v", err))

		if a.UsageError() {
			return 2
		}
		return 1
	}

	return 0
}
