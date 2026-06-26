# Allow standalone build (e.g., from Go tests) or add_subdirectory from root
if(NOT TARGET storeapi)
    cmake_minimum_required(VERSION 3.15)
    project(storeapi_dll LANGUAGES CXX)
    set(CMAKE_CXX_STANDARD 20)
    set(CMAKE_CXX_STANDARD_REQUIRED ON)
    if(MSVC)
        add_definitions(-D_UNICODE)
    endif()
endif()

set(STOREAPI_DIR "${CMAKE_CURRENT_SOURCE_DIR}/..")

set(STOREAPI_DLL_SRCS
    dllmain.cpp
    StoreApi.cpp
    StoreApi.hpp
    framework.hpp
    "${STOREAPI_DIR}/base/impl/StoreContext.cpp"
    "${STOREAPI_DIR}/base/impl/StoreContext.hpp"
    "${STOREAPI_DIR}/base/impl/WinMockContext.cpp"
    "${STOREAPI_DIR}/base/impl/WinMockContext.hpp"
    "${STOREAPI_DIR}/base/Exception.hpp"
    "${STOREAPI_DIR}/base/StoreService.hpp"
    "${STOREAPI_DIR}/agent/ServerStoreService.hpp"
)

add_library(storeapi SHARED ${STOREAPI_DLL_SRCS})

target_include_directories(storeapi PRIVATE "${STOREAPI_DIR}")

target_compile_definitions(storeapi PRIVATE
    STOREAPI_EXPORTS
    _WINDOWS
    _USRDLL
)

if(DEFINED ENV{UP4W_TEST_WITH_MS_STORE_MOCK} OR DEFINED CACHE{UP4W_TEST_WITH_MS_STORE_MOCK})
    target_compile_definitions(storeapi PRIVATE UP4W_TEST_WITH_MS_STORE_MOCK)
endif()

if(MSVC)
    set_target_properties(storeapi PROPERTIES
        VS_WINDOWS_TARGET_PLATFORM_MIN_VERSION 10.0.19041.0
    )
endif()

target_link_libraries(storeapi PRIVATE WindowsApp runtimeobject)
install(TARGETS storeapi RUNTIME DESTINATION ".")
