#include <catch2/catch_session.hpp>
#include <tchar.h>

int _tmain(int argc, _TCHAR* argv[]) {
    // Convert wide args to standard char format for Catch2 session runner
    return Catch::Session().run(argc, reinterpret_cast<char**>(argv));
}
