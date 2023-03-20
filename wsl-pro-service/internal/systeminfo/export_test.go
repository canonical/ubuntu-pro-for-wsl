package systeminfo

import (
	"context"
	"testing"
)

func backup[F any](t *testing.T, f *F) {
	t.Helper()

	backUp := *f
	t.Cleanup(func() {
		*f = backUp
	})
}

// InjectWslRootPath changes the definition of private function wslRootPath
// It is restored during test cleanup.
func InjectWslRootPath(t *testing.T, f func() ([]byte, error)) {
	t.Helper()
	backup(t, &wslRootPath)
	wslRootPath = f
}

// InjectOsRelease changes the definition of private function osRelease
// It is restored during test cleanup.
func InjectOsRelease(t *testing.T, f func() ([]byte, error)) {
	t.Helper()
	backup(t, &osRelease)
	osRelease = f
}

// InjectProStatusCmdOutput changes the definition of private function proStatusCmdOutput
// It is restored during test cleanup.
func InjectProStatusCmdOutput(t *testing.T, f func(context.Context) ([]byte, error)) {
	t.Helper()
	backup(t, &proStatusCmdOutput)
	proStatusCmdOutput = f
}
