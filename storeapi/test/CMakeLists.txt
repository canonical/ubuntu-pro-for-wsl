cmake_minimum_required(VERSION 3.15)

#set(CMAKE_SYSTEM_NAME WindowsStore)
#set(CMAKE_SYSTEM_VERSION 10.0)
set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

project(StoreApiTests LANGUAGES CXX)

if(MSVC)
    add_definitions(-D_UNICODE)
endif()

set(StoreApi_SRCS
    "../base/Context.cpp"
    "../base/Context.hpp"
    "../base/Exception.hpp"
    "../base/StoreService.hpp"
    "../agent/ServerStoreService.cpp"
    "../gui/ClientStoreService.hpp"
)

set(StoreApiTests_SRCS
    "ClientStoreServiceTest.cpp"
    "ServerStoreServiceTest.cpp"
    "StoreServiceTest.cpp"
)

include(FetchContent)

FetchContent_Declare(
  googletest
  URL https://github.com/google/googletest/archive/refs/tags/v1.13.0.zip
)
# For Windows: Prevent overriding the parent project's compiler/linker settings
set(gtest_force_shared_crt ON CACHE BOOL "" FORCE)
# Disable install commands for gtest so it doesn't end up in the bundle.
set(INSTALL_GTEST OFF CACHE BOOL "Disable installation of googletest" FORCE)
FetchContent_MakeAvailable(googletest)
enable_testing()
add_executable(StoreApiTests ${StoreApi_SRCS} ${StoreApiTests_SRCS} )
target_include_directories(StoreApiTests PUBLIC ${CMAKE_CURRENT_LIST_DIR}/.. )
target_compile_features(StoreApiTests PRIVATE cxx_std_20)
set_target_properties(StoreApiTests PROPERTIES VS_WINDOWS_TARGET_PLATFORM_MIN_VERSION 10.0.14393.0 COMPILE_WARNING_AS_ERROR ON)
target_link_libraries(StoreApiTests PRIVATE WindowsApp  GTest::gtest_main)
if(MSVC)
    set(MSVC_OPTIONS
        /W4 # Baseline reasonable warnings
        /w14242 # 'identifier': conversion from 'type1' to 'type1', possible loss of data
        /w14254 # 'operator': conversion from 'type1:field_bits' to 'type2:field_bits', possible loss of data
        /w14263 # 'function': member function does not override any base class virtual member function
        /w14265 # 'classname': class has virtual functions, but destructor is not virtual instances of this class may not
                # be destructed correctly
        /w14287 # 'operator': unsigned/negative constant mismatch
        /we4289 # nonstandard extension used: 'variable': loop control variable declared in the for-loop is used outside
                # the for-loop scope
        /w14296 # 'operator': expression is always 'boolean_value'
        /w14311 # 'variable': pointer truncation from 'type1' to 'type2'
        /w14545 # expression before comma evaluates to a function which is missing an argument list
        /w14546 # function call before comma missing argument list
        /w14547 # 'operator': operator before comma has no effect; expected operator with side-effect
        /w14549 # 'operator': operator before comma has no effect; did you intend 'operator'?
        /w14555 # expression has no effect; expected expression with side- effect
        /w14619 # pragma warning: there is no warning number 'number'
        /w14640 # Enable warning on thread un-safe static member initialization
        /w14826 # Conversion from 'type1' to 'type_2' is sign-extended. This may cause unexpected runtime behavior.
        /w14905 # wide string literal cast to 'LPSTR'
        /w14906 # string literal cast to 'LPWSTR'
        /w14928 # illegal copy-initialization; more than one user-defined conversion has been implicitly applied
        /permissive- # standards conformance mode for MSVC compiler.
        /fsanitize=address,leak,undefined
        /Zi 
        /INCREMENTAL:NO
        /await
    )
    target_compile_options(StoreApiTests INTERFACE ${MSVC_OPTIONS})
 endif()

include(GoogleTest)
gtest_discover_tests(StoreApiTests)
