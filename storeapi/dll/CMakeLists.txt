# Allow standalone build (e.g., from Go tests) or add_subdirectory from root
cmake_minimum_required(VERSION 3.25)
project(storeapi_dll LANGUAGES CXX)

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
target_compile_features(storeapi PRIVATE cxx_std_20)
target_include_directories(storeapi PRIVATE "${STOREAPI_DIR}")

target_compile_definitions(storeapi PRIVATE
    STOREAPI_EXPORTS
)

# Should only be needed on standalone builds. If included via add_subdirectory() that definition should be inherited instead.
if(PROJECT_IS_TOP_LEVEL)
    if(MSVC)
        target_compile_definitions(storeapi PRIVATE _UNICODE)
    endif()
endif()

option(UP4W_TEST_WITH_MS_STORE_MOCK "Use mock store API" OFF)
if(UP4W_TEST_WITH_MS_STORE_MOCK OR DEFINED ENV{UP4W_TEST_WITH_MS_STORE_MOCK})
    target_compile_definitions(storeapi PRIVATE UP4W_TEST_WITH_MS_STORE_MOCK)
endif()

target_link_libraries(storeapi PRIVATE WindowsApp runtimeobject)
install(TARGETS storeapi RUNTIME DESTINATION ".")
