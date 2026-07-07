set(LAUNCHER_SRCS
    main.cpp
    console.cpp
    console.hpp
    error.cpp
    error.hpp
)

add_executable(ubuntu-pro-agent-launcher WIN32 ${LAUNCHER_SRCS})

target_compile_features(ubuntu-pro-agent-launcher PRIVATE cxx_std_20)

target_link_libraries(ubuntu-pro-agent-launcher PRIVATE WindowsApp)
install(TARGETS ubuntu-pro-agent-launcher DESTINATION ".")
