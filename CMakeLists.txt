cmake_minimum_required(VERSION 3.25)

# C++ component build (CMake). For the full build, use: .\build.ps1 -build
project(UP4W LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

if(MSVC)
    add_compile_definitions($<$<COMPILE_LANGUAGE:CXX>:_UNICODE>)
endif()

add_subdirectory(storeapi/dll)
add_subdirectory(storeapi/test EXCLUDE_FROM_ALL)
add_subdirectory(launcher)

enable_testing()

# Force CMake to install VC Runtime libs for debug only.
# DLLs to be placed at <prefix>/../VFS/SystemX64 while other components will be directly placed at <prefix>.
set(CMAKE_INSTALL_DEBUG_LIBRARIES TRUE)
set(CMAKE_INSTALL_UCRT_LIBRARIES TRUE)
set(CMAKE_INSTALL_DEBUG_LIBRARIES_ONLY TRUE)
set(CMAKE_INSTALL_SYSTEM_RUNTIME_LIBS_SKIP TRUE)
include(InstallRequiredSystemLibraries)
install(PROGRAMS ${CMAKE_INSTALL_SYSTEM_RUNTIME_LIBS} DESTINATION "${CMAKE_INSTALL_PREFIX}/../VFS/SystemX64" CONFIGURATIONS Debug)
