#include "security.hpp"

#include <catch2/catch_test_macros.hpp>

using efivar::service::SecurityDescriptor;

TEST_CASE("Create builds an AU-based security descriptor", "[security]") {
    auto result = SecurityDescriptor::Create();
    REQUIRE(result.has_value());
    REQUIRE(result->get() != nullptr);
    REQUIRE(result->get()->lpSecurityDescriptor != nullptr);
}

TEST_CASE("FromCurrentUser builds a user-based security descriptor", "[security]") {
    auto result = SecurityDescriptor::FromCurrentUser();
    REQUIRE(result.has_value());
    REQUIRE(result->get() != nullptr);
    REQUIRE(result->get()->lpSecurityDescriptor != nullptr);
}

TEST_CASE("Security descriptor is usable for pipe creation", "[security]") {
    auto result = SecurityDescriptor::Create();
    REQUIRE(result.has_value());
    REQUIRE(result->get()->nLength == sizeof(SECURITY_ATTRIBUTES));
    REQUIRE_FALSE(result->get()->bInheritHandle);
}
