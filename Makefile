# This high level Makefile is a contract with TiCS to make our non-trivial build system
# becomes a simple one-liner for the TiCS build pipeline. It is not intended to be used by CI,
# but to replicate what CI does in a way TiCS can see. It might still be useful for local development.
# It assumes GNU Make, and will not work with NMAKE provided by Visual Studio.

all: tics_build


# Prepares a build tree and executes static code analysis on the C++ code via Visual Studio integration.
# Because of how Visual Studio implements SCA, we have to run it to completion in order to find the `compile_commands.json` files it generates, one per target.
# Examples of folders where we can find a compilation database are:
#  - \build\windows-msvc-sca\launcher\ubuntu-pro-agent-launcher.dir\Debug\ubuntu-pro-agent-launcher.ClangTidy\
#  - \build\windows-msvc-sca\storeapi\dll\Debug\storeapi.ClangTidy\
tics_analyze:
	cmake --preset windows-msvc-sca
	cmake --build --preset windows-msvc-sca --verbose


# Groups all compilation commands used in our CI so TiCS can have full visibility of our build process from a single command.
tics_build:
	powershell.exe -ExecutionPolicy Bypass -File build.ps1 -Action Compile -Verbose -Config Debug -Version 0.0.0.0 -Tag 0.0.0
	cmake -E make_directory build\tests
	cmake -S . -B build\tests
	cmake --build build\tests --config Debug --verbose


.PHONY: clean
clean:
ifeq ($(SHELLTYPE), sh)
	rm -rf build
else
	del /q build
endif
