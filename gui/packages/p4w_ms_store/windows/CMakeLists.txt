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

# Setting up WinRT support
# WinRT relies on either header files shipped with the Windows SDK or header file generation through the cppwinrt.exe
# program, which can be acquired, among other ways, through NuGet.
# This is the same approach implemented in the Windows implementation of the (Flutter "official") local_auth plugin
# See https://github.com/flutter/packages/blob/main/packages/local_auth/local_auth_windows/windows/CMakeLists.txt
include(FetchContent)
# Visual Studio doesn't ship nuget.exe binary by default.
FetchContent_Declare(nuget
  URL "https://dist.nuget.org/win-x86-commandline/v6.0.0/nuget.exe"
  URL_HASH SHA256=04eb6c4fe4213907e2773e1be1bbbd730e9a655a3c9c58387ce8d4a714a5b9e1
  DOWNLOAD_NO_EXTRACT true
)
# Just in case we try first to find it in PATH.
find_program(NUGET nuget)
if (NOT NUGET)
    message(VERBOSE "Nuget.exe not found, trying to download or use cached version.")
    FetchContent_MakeAvailable(nuget)
    set(NUGET ${nuget_SOURCE_DIR}/nuget.exe)
endif()
# The cppwinrt.exe version chosen is the same of the [local_auth] plugin.
# It's lower than the one shipped with SDK version 10.0.26100.0 (the current SDK version).
# The only machine-level side effect of this is the download of CppWinRT's nuget file to
# Nuget's cache (~.nuget\packages), and the unzipping of it to that same folder
set(CPPWINRT_VERSION "2.0.220418.1")
execute_process(COMMAND
    ${NUGET} install Microsoft.Windows.CppWinRT -Version ${CPPWINRT_VERSION} -OutputDirectory packages
    WORKING_DIRECTORY ${CMAKE_BINARY_DIR}
    RESULT_VARIABLE ret)
if (NOT ret EQUAL 0)
    message(FATAL_ERROR "Failed to install nuget package Microsoft.Windows.CppWinRT.${CPPWINRT_VERSION}")
endif()
set(CPPWINRT ${CMAKE_BINARY_DIR}/packages/Microsoft.Windows.CppWinRT.${CPPWINRT_VERSION}/bin/cppwinrt.exe)
# Note that the header files are generated inside the CMake build tree.
execute_process(COMMAND
    ${CPPWINRT} -input sdk -output include
    WORKING_DIRECTORY ${CMAKE_BINARY_DIR}
    RESULT_VARIABLE ret)
if (NOT ret EQUAL 0)
    message(FATAL_ERROR "Failed to run cppwinrt.exe")
endif()
# To allow including WinRT headers.
# target_include_directories cannot specify include directories for target "BEFORE"
# on a subproject, thus this has to use the global include_directories.
include_directories(BEFORE SYSTEM ${CMAKE_BINARY_DIR}/include)

# Relative to the top level cmake project which will be either this plugin or the app
# consuming it. Either way, same relative distance to the directory we are looking for.
set(STORE_API_DEFINES "")
set(STORE_API_DIR "${CMAKE_SOURCE_DIR}/../../../../storeapi/")
set(STORE_API_SRC
    "${STORE_API_DIR}/gui/ClientStoreService.hpp"
    "${STORE_API_DIR}/base/StoreService.hpp"
    "${STORE_API_DIR}/base/Purchase.hpp"
    "${STORE_API_DIR}/base/Exception.hpp"
    "${STORE_API_DIR}/base/DefaultContext.hpp"

    # API wrapper implementations
    "${STORE_API_DIR}/base/impl/StoreContext.cpp"
    "${STORE_API_DIR}/base/impl/StoreContext.hpp"
    "${STORE_API_DIR}/base/impl/WinMockContext.cpp"
    "${STORE_API_DIR}/base/impl/WinMockContext.hpp"
)
if(DEFINED ENV{UP4W_TEST_WITH_MS_STORE_MOCK})
    # TODO: Change to informative warning and update the text once mock client API wrappers are avaiable.
    message(WARNING "Building the MS Store plugin with the mock client API due environment variable 'UP4W_TEST_WITH_MS_STORE_MOCK' set to '$ENV{UP4W_TEST_WITH_MS_STORE_MOCK}'.")
    list(APPEND STORE_API_DEFINES "UP4W_TEST_WITH_MS_STORE_MOCK")
else()
    message(STATUS "Building with the production version of MS Store client API. Set the environment variable 'UP4W_TEST_WITH_MS_STORE_MOCK' if you want to build with the mock store API.")
endif()

# Any new source files that you add to the plugin should be added here.
list(APPEND PLUGIN_SOURCES
  "p4w_ms_store_plugin.cpp"
  "p4w_ms_store_plugin.h"
  "p4w_ms_store_plugin_impl.cpp"
  "p4w_ms_store_plugin_impl.h"
  ${STORE_API_SRC}
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
target_compile_definitions(${PLUGIN_NAME} PRIVATE FLUTTER_PLUGIN_IMPL)
# Further #defines we may want to pass.
target_compile_definitions(${PLUGIN_NAME} PRIVATE ${STORE_API_DEFINES})

# Source include directories and library dependencies. Add any plugin-specific
# dependencies here.
target_include_directories(${PLUGIN_NAME} INTERFACE
  "${CMAKE_CURRENT_SOURCE_DIR}/include" PRIVATE ${STORE_API_DIR})
target_compile_features(${PLUGIN_NAME} PRIVATE cxx_std_20)
target_compile_options(${PLUGIN_NAME} PRIVATE /await)
target_link_libraries(${PLUGIN_NAME} PRIVATE flutter flutter_wrapper_plugin windowsapp)

# List of absolute paths to libraries that should be bundled with the plugin.
# This list could contain prebuilt libraries, or libraries created by an
# external build triggered from this build file.
set(p4w_ms_store_bundled_libraries
  ""
  PARENT_SCOPE
)
