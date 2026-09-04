include(FetchContent)

FetchContent_Declare(
    wil
    GIT_REPOSITORY https://github.com/microsoft/wil.git
    GIT_TAG v1.0.240803.1
    EXCLUDE_FROM_ALL
)
set(WIL_BUILD_TESTS OFF CACHE BOOL "" FORCE)
set(WIL_BUILD_PACKING OFF CACHE BOOL "" FORCE)
FetchContent_MakeAvailable(wil)
FetchContent_Declare(
    Catch2
    GIT_REPOSITORY https://github.com/catchorg/Catch2.git
    GIT_TAG v3.7.0
    EXCLUDE_FROM_ALL
)
set(CATCH_BUILD_TESTING OFF CACHE BOOL "" FORCE)
set(CATCH_INSTALL_DOCS OFF CACHE BOOL "" FORCE)
FetchContent_MakeAvailable(Catch2)

add_executable(ubuntu-pro-service main.cpp)
target_compile_features(ubuntu-pro-service PRIVATE cxx_std_23)
target_compile_definitions(ubuntu-pro-service PRIVATE UNICODE _UNICODE)
target_include_directories(ubuntu-pro-service PRIVATE ${CMAKE_CURRENT_SOURCE_DIR})
target_link_libraries(ubuntu-pro-service PRIVATE WIL::WIL ntdll)
target_sources(ubuntu-pro-service PRIVATE
    app.hpp
    pipe.hpp
    token.hpp
    security.hpp
    firmware.hpp
    eventlog.hpp
    service_control.hpp
)

add_executable(service-tests
    tests/protocol_test.cpp
    tests/token_test.cpp
    tests/security_test.cpp
    tests/pipe_test.cpp
    tests/firmware_test.cpp
    tests/test_main.cpp
)
target_compile_features(service-tests PRIVATE cxx_std_23)
target_compile_definitions(service-tests PRIVATE UNICODE _UNICODE)
target_include_directories(service-tests PRIVATE ${CMAKE_CURRENT_SOURCE_DIR} ${wil_SOURCE_DIR}/include)
target_link_libraries(service-tests PRIVATE Catch2::Catch2 WIL::WIL ntdll)

install(TARGETS ubuntu-pro-service RUNTIME DESTINATION ../service COMPONENT ubuntu-pro-service)