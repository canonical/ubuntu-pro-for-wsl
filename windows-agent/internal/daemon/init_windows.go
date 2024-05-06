//go:build !gowslmock

package daemon

func init() {
	defaultOptions = options{
		wslSystemCmd:         []string{"wsl.exe", "--system", "wslinfo", "--networking-mode", "-n"},
		getAdaptersAddresses: getWindowsAdaptersAddresses,
	}
}
