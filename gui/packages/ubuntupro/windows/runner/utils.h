#ifndef RUNNER_UTILS_H_
#define RUNNER_UTILS_H_

#include <string>
#include <vector>

// Creates a console for the process, and redirects stdout and stderr to
// it for both the runner and the Flutter library.
void CreateAndAttachConsole();

// Conditionally arranges the console output so that we preserve the default behavior when started by the flutter tool
// or by a debugger, add add a new behavior for when started by a console shell: resync stdio so the outputs are
// visible in the parent console. Useful for end-to-end tests (as well as for apps intended to be started by both a
// desktop and console shells).
//
// In a nutshell:
// 1. If started by the Flutter tool (which is via CLI), it attaches to the parent console and redirects its output so
// the desktop device log reader can consume its outputs.
// 2. If started by a debugger (which is usually not via CLI on Windows), i.e. creates a new console and and redirects
// its output so the desktop device log reader can consume its outputs.
// 3. If started by a shell (console, but not the flutter tool), attaches to the parent console and resync stdio so the
// outputs are visible in the parent console, since there is no log reader in this context.
void SetupConsole();

// Takes a null-terminated wchar_t* encoded in UTF-16 and returns a std::string
// encoded in UTF-8. Returns an empty std::string on failure.
std::string Utf8FromUtf16(const wchar_t* utf16_string);

// Gets the command line arguments passed in as a std::vector<std::string>,
// encoded in UTF-8. Returns an empty std::vector<std::string> on failure.
std::vector<std::string> GetCommandLineArguments();

#endif  // RUNNER_UTILS_H_
