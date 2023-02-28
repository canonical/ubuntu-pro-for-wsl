package task

import "testing"

func BackupRegistry(t *testing.T) {
	t.Helper()

	backup := registry
	registry = make(map[string]decodeFunc)

	t.Cleanup(func() {
		registry = backup
	})
}

func RegisteredTasks() (out []string) {
	for k := range registry {
		out = append(out, k)
	}
	return out
}
