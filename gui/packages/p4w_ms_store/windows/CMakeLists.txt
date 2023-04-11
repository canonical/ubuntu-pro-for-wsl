# The Flutter tooling requires that developers have a version of Visual Studio
# installed that includes CMake 3.14 or later. You should not increase this
# version, as doing so will cause the plugin to fail to compile for some
# customers of the plugin.
cmake_minimum_required(VERSION 3.14)

# Project-level configuration.
set(PROJECT_NAME "p4w_ms_store")
project(${PROJECT_NAME} LANGUAGES CXX)

# This value is used when generating builds using this plugin, so it must
# not be changed
set(PLUGIN_NAME "p4w_ms_store_plugin")

## Setting up the WinRT dependency support:
## CppWinRT stuff starts here. =======================================================================================================================
## Using the same NuGet and CppWinRT versions used in the official Flutter LocalAuth plugin.
set(CPPWINRT_VERSION "2.0.220418.1")
include(FetchContent)
FetchContent_Declare(nuget
  URL "https://dist.nuget.org/win-x86-commandline/v6.0.0/nuget.exe"
  URL_HASH SHA256=04eb6c4fe4213907e2773e1be1bbbd730e9a655a3c9c58387ce8d4a714a5b9e1
  DOWNLOAD_NO_EXTRACT true
)

# Finds or fetches NuGet.
find_program(NUGET nuget)
if (NOT NUGET)
    message(VERBOSE "Nuget.exe not found, trying to download or use cached version.")
    FetchContent_MakeAvailable(nuget)
    set(NUGET ${nuget_SOURCE_DIR}/nuget.exe)
endif()

# Installs CppWinRT via NuGet.
execute_process(COMMAND
    ${NUGET} install Microsoft.Windows.CppWinRT -Version ${CPPWINRT_VERSION} -OutputDirectory packages
    WORKING_DIRECTORY ${CMAKE_BINARY_DIR}
    RESULT_VARIABLE ret)
if (NOT ret EQUAL 0)
    message(FATAL_ERROR "Failed to install nuget package Microsoft.Windows.CppWinRT.${CPPWINRT_VERSION}")
endif()

## cppwinrt.exe won't generate bindings for extension SDK's by default, thus we need to find the metadata file (.winmd)
## and pass it explicitly as an `-input`. This looks for the `Windows.ApplicationModel.FullTrustAppContract.winmd`
## metadata file in the active Windows SDK dir and stop building if it cannot find it.
if(DEFINED ENV{WindowsSdkDir})
    set(FULLTRUST_WINMD "Windows.ApplicationModel.FullTrustAppContract")
    set(SDK_WINMD_DIR "$ENV{WindowsSdkDir}References\\$ENV{WindowsSDKLibVersion}")

    FILE(GLOB_RECURSE FULLTRUST_WINMD_PATH LIST_DIRECTORIES false "${SDK_WINMD_DIR}${FULLTRUST_WINMD}/*/${FULLTRUST_WINMD}.winmd")
else()
    message(FATAL_ERROR "Cannot find the required .winmd files. Make sure to run flutter from either the Developer cmd or powershell.")
endif()
if (NOT EXISTS ${FULLTRUST_WINMD_PATH})
    message(FATAL_ERROR "Additional API Contracts are required but not accessible: \"${FULLTRUST_WINMD}.winmd\" not found in \"${SDK_WINMD_DIR}\"")
endif()

## Runs cppwinrt.exe to generate the Windows Runtime C++ bindings (projections).
set(CPPWINRT ${CMAKE_BINARY_DIR}/packages/Microsoft.Windows.CppWinRT.${CPPWINRT_VERSION}/bin/cppwinrt.exe)
execute_process(COMMAND
    ${CPPWINRT} -input sdk -input ${FULLTRUST_WINMD_PATH} -output include
    WORKING_DIRECTORY ${CMAKE_BINARY_DIR}
    RESULT_VARIABLE ret)
if (NOT ret EQUAL 0)
    message(FATAL_ERROR "Failed to run cppwinrt.exe")
endif()
## Adds the CppWinRT output directory to the include path.
include_directories(BEFORE SYSTEM ${CMAKE_BINARY_DIR}/include)

## CppWinRT stuff ends here. =======================================================================================================================

# Any new source files that you add to the plugin should be added here.
list(APPEND PLUGIN_SOURCES
  "p4w_ms_store_plugin.cpp"
  "p4w_ms_store_plugin.h"
# The sources we added
  "p4w_channel_constants.h"
  "p4w_methods.cpp"
  "p4w_methods.h"
  "p4w_winrt_api.cpp"
  "p4w_winrt_api.h"
)

# Define the plugin library target. Its name must not be changed (see comment
# on PLUGIN_NAME above).
add_library(${PLUGIN_NAME} SHARED
  "include/p4w_ms_store/p4w_ms_store_plugin_c_api.h"
  "p4w_ms_store_plugin_c_api.cpp"
  ${PLUGIN_SOURCES}
)

# Apply a standard set of build settings that are configured in the
# application-level CMakeLists.txt. This can be removed for plugins that want
# full control over build settings.
apply_standard_settings(${PLUGIN_NAME})

# Symbols are hidden by default to reduce the chance of accidental conflicts
# between plugins. This should not be removed; any symbols that should be
# exported should be explicitly exported with the FLUTTER_PLUGIN_EXPORT macro.
set_target_properties(${PLUGIN_NAME} PROPERTIES
  CXX_VISIBILITY_PRESET hidden)

## Enabling C++20 for WinRT
target_compile_features(${PLUGIN_NAME} PRIVATE cxx_std_20)
## Notice that the WinRT generated bindings for C++ are header-only,
## thus, no `target_link_libraries(...)`.

target_compile_definitions(${PLUGIN_NAME} PRIVATE FLUTTER_PLUGIN_IMPL)

# Source include directories and library dependencies. Add any plugin-specific
# dependencies here.
target_include_directories(${PLUGIN_NAME} INTERFACE
  "${CMAKE_CURRENT_SOURCE_DIR}/include")
target_link_libraries(${PLUGIN_NAME} PRIVATE flutter flutter_wrapper_plugin)

# List of absolute paths to libraries that should be bundled with the plugin.
# This list could contain prebuilt libraries, or libraries created by an
# external build triggered from this build file.
set(p4w_ms_store_bundled_libraries
  ""
  PARENT_SCOPE
)

# === Native Code Tests ===
#
# The current best practice for building plugin native tests is to trigger the
# build from the example application, usually part of the plugin repository.
# Since our sample app is the `ubuntupro` which is the only client this plugin
# is ever supposed to have, we'll build the plugin tests based on the presence
# of the `INCLUDE_PRO_PLUGIN_TESTS` environment variable at configuration time.
#
# On Linux Shell `export INCLUDE_PRO_PLUGIN_TESTS=1` and `unset INCLUDE_PRO_PLUGIN_TESTS`
# On Windows Powershell set `$env:INCLUDE_PRO_PLUGIN_TESTS=1` and unset it with
# `$env:INCLUDE_PRO_PLUGIN_TESTS=$null`.
# This way CMake will see it.
#

if (DEFINED ENV{INCLUDE_PRO_PLUGIN_TESTS})
    enable_testing()
    message(STATUS "Building ${PROJECT} tests")
    FetchContent_Declare(
    googletest
    URL https://github.com/google/googletest/archive/release-1.11.0.zip
    )
    # Prevent overriding the parent project's compiler/linker settings
    set(gtest_force_shared_crt ON CACHE BOOL "" FORCE)
    # Disable install commands for gtest so it doesn't end up in the bundle.
    set(INSTALL_GTEST OFF CACHE BOOL "Disable installation of googletest" FORCE)

    FetchContent_MakeAvailable(googletest)

    set(TEST_RUNNER "${PROJECT_NAME}_test")
    # The plugin's C API is not very useful for unit testing, so build the sources
    # directly into the test binary rather than using the DLL.
    add_executable(${TEST_RUNNER}
    #   mocks.h
    test/mocks.h
    test/launch_test.cpp
    ${PLUGIN_SOURCES}
    )
    apply_standard_settings(${TEST_RUNNER})
    target_include_directories(${TEST_RUNNER} PRIVATE "${CMAKE_CURRENT_SOURCE_DIR}")
    target_compile_features(${TEST_RUNNER} PRIVATE cxx_std_20)
    target_link_libraries(${TEST_RUNNER} PRIVATE flutter_wrapper_plugin)
    target_link_libraries(${TEST_RUNNER} PRIVATE gtest_main gmock)

    # flutter_wrapper_plugin has link dependencies on the Flutter DLL.
    add_custom_command(TARGET ${TEST_RUNNER} POST_BUILD
    COMMAND ${CMAKE_COMMAND} -E copy_if_different
    "${FLUTTER_LIBRARY}" $<TARGET_FILE_DIR:${TEST_RUNNER}>
    )

    include(GoogleTest)
    gtest_discover_tests(${TEST_RUNNER})
endif()