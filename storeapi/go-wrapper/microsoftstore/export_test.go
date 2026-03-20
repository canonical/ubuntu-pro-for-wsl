package microsoftstore

// FindWorkspaceRoot climbs up the current working directory until the Go workspace root is found.
var FindWorkspaceRoot = findWorkspaceRoot

// CheckError inspects the values of hres and err to determine what kind of error we have, if any, according to the rules of syscall/dll_windows.go.
var CheckError = checkError

// WithLoadDLLFailure sets the LoadError of the LazyDLL to the given error, simulating a failure to
// load the DLL on Linux. This is used for testing purposes.
var WithLoadDLLFailure = withLoadDLLFailure

// WithFindProcFailure sets the FindError of the LazyProc to the given error, simulating a failure to
// find the procedure in the DLL on Linux. This is used for testing purposes.
var WithFindProcFailure = withFindProcFailure

// WithCallProcFailure sets the CallResult error of the LazyProc to the given error, simulating a failure to
// call the procedure in the DLL on Linux. This is used for testing purposes.
var WithCallProcFailure = withCallProcFailure

// ResetErrors resets all programmed failure modes.
func ResetErrors() {
	reset()
}
