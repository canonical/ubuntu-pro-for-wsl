set(LAUNCHER_SRCS
    main.cpp
    console.cpp
    console.hpp
    error.cpp
    error.hpp
)

add_executable(ubuntu-pro-agent-launcher WIN32 ${LAUNCHER_SRCS})

if(MSVC)
    set_target_properties(ubuntu-pro-agent-launcher PROPERTIES
        VS_WINDOWS_TARGET_PLATFORM_MIN_VERSION 10.0.19041.0
    )
endif()

target_link_libraries(ubuntu-pro-agent-launcher PRIVATE WindowsApp)
install(TARGETS ubuntu-pro-agent-launcher DESTINATION ".")
