package microsoftstore

// FindWorkspaceRoot climbs up the current working directory until the Go workspace root is found.
var FindWorkspaceRoot = findWorkspaceRoot

// CheckError inspects the values of hres and err to determine what kind of error we have, if any, according to the rules of syscall/dll_windows.go.
var CheckError = checkError
