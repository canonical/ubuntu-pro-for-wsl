package testutils

import (
	"os"
	"path/filepath"
)

// MockSecureReader is a test mock implementing daemon.SecureReader.
type MockSecureReader struct {
	readFileFunc func(rootDir, targetPath string) ([]byte, error)
}

// ReadFile calls the behavior configured at construction time.
func (m *MockSecureReader) ReadFile(rootDir, targetPath string) ([]byte, error) {
	return m.readFileFunc(rootDir, targetPath)
}

// NewMockSecureReader creates a MockSecureReader with the given behavior.
// A nil fn makes ReadFile delegate to os.ReadFile on the joined path, bypassing validation.
func NewMockSecureReader(fn func(rootDir, targetPath string) ([]byte, error)) *MockSecureReader {
	if fn == nil {
		fn = func(rootDir, targetPath string) ([]byte, error) {
			return os.ReadFile(filepath.Join(rootDir, targetPath)) //#nosec G304 // Test mock intentionally reads the requested path.
		}
	}
	return &MockSecureReader{readFileFunc: fn}
}
