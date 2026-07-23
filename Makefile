# This high level Makefile is a contract with TiCS to make our non-trivial build system
# becomes a simple one-liner for the TiCS build pipeline. It is not intended to be used by CI,
# but to replicate what CI does in a way TiCS can see. It might still be useful for local development.

all: tics_analyze tics_build


tics_analyze:
	cmake --preset windows-msvc-sca
	cmake --build --preset windows-msvc-sca --verbose

tics_build:
	powershell.exe -ExecutionPolicy Bypass -File build.ps1 -Action Compile -Verbose -Config Debug -Version 0.0.0.0 -Tag 0.0.0
	cmake -E make_directory build_tests
	cmake -S . -B build_tests
	cmake --build build_tests --config Debug --verbose

