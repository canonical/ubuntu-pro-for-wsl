#include "protocol.h"

#include <catch2/catch_test_macros.hpp>

TEST_CASE("Request has expected size", "[protocol]") {
    static_assert(sizeof(efivar::Request) == 8);
}

TEST_CASE("Response has expected size", "[protocol]") {
    // Current protocol.h defines Response with five packed fields: 2+2+4+4+4 = 16.
    static_assert(sizeof(efivar::Response) == 16);
}

TEST_CASE("ServiceErrc values match wire values", "[protocol]") {
    REQUIRE(static_cast<std::uint32_t>(efivar::ServiceErrc::Success) == 0);
    REQUIRE(static_cast<std::uint32_t>(efivar::ServiceErrc::PrivilegeFailed) == 1);
    REQUIRE(static_cast<std::uint32_t>(efivar::ServiceErrc::FirmwareReadFailed) == 2);
    REQUIRE(static_cast<std::uint32_t>(efivar::ServiceErrc::BadRequest) == 3);
    REQUIRE(static_cast<std::uint32_t>(efivar::ServiceErrc::ConnectionRefused) == 4);
}
