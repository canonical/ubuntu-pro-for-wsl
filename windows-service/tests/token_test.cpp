#include "token.hpp"

#include <catch2/catch_test_macros.hpp>

using efivar::service::TokenPrivilege;

TEST_CASE("Acquire enables a valid privilege", "[token]") {
    auto result = TokenPrivilege::Acquire(SE_SYSTEM_ENVIRONMENT_NAME);
    if (result.has_value()) {
        REQUIRE(result->enabled());
    } else {
        // The privilege may not be present in the test process token.
        REQUIRE(result.error().value() == ERROR_NOT_ALL_ASSIGNED);
    }
}

TEST_CASE("Acquire fails for an invalid privilege", "[token]") {
    auto result = TokenPrivilege::Acquire(L"NonExistentPrivilege");
    REQUIRE_FALSE(result.has_value());
}

TEST_CASE("Destructor disables the privilege", "[token]") {
    {
        auto result = TokenPrivilege::Acquire(SE_SYSTEM_ENVIRONMENT_NAME);
        if (!result.has_value()) {
            SKIP("Privilege not available in test token");
        }
    }
    // After destruction the privilege should be disabled.
    auto result = TokenPrivilege::Acquire(SE_SYSTEM_ENVIRONMENT_NAME);
    REQUIRE(result.has_value());
}

TEST_CASE("TokenPrivilege is movable", "[token]") {
    auto result = TokenPrivilege::Acquire(SE_SYSTEM_ENVIRONMENT_NAME);
    if (!result.has_value()) {
        SKIP("Privilege not available in test token");
    }

    TokenPrivilege moved = std::move(*result);
    REQUIRE(moved.enabled());
    REQUIRE_FALSE(result->enabled());
}
