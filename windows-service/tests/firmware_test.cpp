#include "firmware.hpp"

#include <vector>
#include <catch2/catch_test_macros.hpp>

using namespace efivar::service::Firmware;

namespace {

    struct EfiVariableHeader {
        ULONG NextEntryOffset;
        ULONG ValueOffset;
        ULONG ValueLength;
        ULONG Attributes;
        GUID VendorGuid;
    };

std::vector<std::uint8_t> BuildBuffer() {
    // Entry 1: "TestVar\0", value {0x01, 0x02, 0x03}
    // Entry 2: "Other\0",   value {0xAA, 0xBB}
    constexpr size_t kHeader = sizeof(EfiVariableHeader);
    const wchar_t kName1[] = L"TestVar";
    constexpr size_t kName1Bytes = 8 * sizeof(wchar_t);
    const wchar_t kName2[] = L"Other";
    constexpr size_t kName2Bytes = 6 * sizeof(wchar_t);

    constexpr ULONG kValueOffset1 = kHeader + kName1Bytes;
    constexpr ULONG kValueOffset2 = kHeader + kName2Bytes;
    constexpr size_t kValue1Size = 3;
    constexpr size_t kValue2Size = 2;
    constexpr ULONG kEntry1Size = kValueOffset1 + kValue1Size;
    constexpr ULONG kEntry2Size = kValueOffset2 + kValue2Size;
    constexpr EfiVariableHeader EfiVariableHeader1 = {
        .NextEntryOffset = kEntry1Size,
        .ValueOffset = kValueOffset1,
        .ValueLength = kValue1Size,
        .Attributes = 0,
        .VendorGuid = {0x11111111, 0x2222, 0x3333, {0x44,0x55,0x66,0x77, 0x88,0x99,0xAA,0xBB}}
    };
    constexpr EfiVariableHeader EfiVariableHeader2 = {
        .NextEntryOffset = 0,
        .ValueOffset = kValueOffset2,
        .ValueLength = kValue2Size,
        .Attributes = 0,
        .VendorGuid = {0xAAAAAAAA, 0xBBBB, 0xCCCC, {0xDD,0xEE,0xFF,0x00, 0x11,0x22,0x33,0x44}}
    };

    
    std::vector<std::uint8_t> buf;
    buf.reserve(kEntry1Size + kEntry2Size);
    
    auto pushHeader = [&](const EfiVariableHeader& h) {
        buf.insert(buf.end(), reinterpret_cast<const std::uint8_t*>(&h), reinterpret_cast<const std::uint8_t*>(&h) + sizeof(h));
    };

    // --- Entry 1 ---
    pushHeader(EfiVariableHeader1);
    
    buf.insert(buf.end(), reinterpret_cast<const std::uint8_t*>(kName1),
               reinterpret_cast<const std::uint8_t*>(kName1) + kName1Bytes);
    buf.push_back(0x01); buf.push_back(0x02); buf.push_back(0x03);

    // --- Entry 2 (last) ---
    pushHeader(EfiVariableHeader2);

    buf.insert(buf.end(), reinterpret_cast<const std::uint8_t*>(kName2),
               reinterpret_cast<const std::uint8_t*>(kName2) + kName2Bytes);
    buf.push_back(0xAA); buf.push_back(0xBB);

    return buf;
}

struct EfiNameHeader {
    ULONG NextEntryOffset;
    GUID VendorGuid;
};

std::vector<std::uint8_t> BuildNameBuffer() {
    // Entry 1: "TestVar\0"
    // Entry 2: "Other\0" (last)
    constexpr size_t kHeader = sizeof(EfiNameHeader);
    const wchar_t kName1[] = L"TestVar";
    constexpr size_t kName1Bytes = 8 * sizeof(wchar_t);
    const wchar_t kName2[] = L"Other";
    constexpr size_t kName2Bytes = 6 * sizeof(wchar_t);

    constexpr ULONG kEntry1Size = static_cast<ULONG>(kHeader + kName1Bytes);
    constexpr ULONG kEntry2Size = static_cast<ULONG>(kHeader + kName2Bytes);
    constexpr EfiNameHeader EfiNameHeader1 = {
        .NextEntryOffset = kEntry1Size,
        .VendorGuid = {0x11111111, 0x2222, 0x3333, {0x44,0x55,0x66,0x77, 0x88,0x99,0xAA,0xBB}}
    };
    constexpr EfiNameHeader EfiNameHeader2 = {
        .NextEntryOffset = 0,
        .VendorGuid = {0xAAAAAAAA, 0xBBBB, 0xCCCC, {0xDD,0xEE,0xFF,0x00, 0x11,0x22,0x33,0x44}}
    };

    std::vector<std::uint8_t> buf;
    buf.reserve(kEntry1Size + kEntry2Size);

    auto pushHeader = [&](const EfiNameHeader& h) {
        buf.insert(buf.end(), reinterpret_cast<const std::uint8_t*>(&h), reinterpret_cast<const std::uint8_t*>(&h) + sizeof(h));
    };

    // --- Entry 1 ---
    pushHeader(EfiNameHeader1);
    buf.insert(buf.end(), reinterpret_cast<const std::uint8_t*>(kName1),
               reinterpret_cast<const std::uint8_t*>(kName1) + kName1Bytes);

    // --- Entry 2 (last) ---
    pushHeader(EfiNameHeader2);
    buf.insert(buf.end(), reinterpret_cast<const std::uint8_t*>(kName2),
               reinterpret_cast<const std::uint8_t*>(kName2) + kName2Bytes);

    return buf;
}

} // namespace

TEST_CASE("EfiVariableIterator iterates two entries", "[firmware]") {
    auto buf = BuildBuffer();
    EfiVariableRange range(buf.data(), buf.data() + buf.size());

    auto it = range.begin();
    auto end = range.end();

    REQUIRE(it != end);
    REQUIRE(std::wcscmp(it->name.data(), L"TestVar") == 0);
    REQUIRE(it->guid->Data1 == 0x11111111);
    REQUIRE(it->guid->Data2 == 0x2222);
    REQUIRE(it->guid->Data3 == 0x3333);
    REQUIRE(it->value.size() == 3);
    REQUIRE(static_cast<unsigned>(it->value[0]) == 0x01);
    REQUIRE(static_cast<unsigned>(it->value[1]) == 0x02);
    REQUIRE(static_cast<unsigned>(it->value[2]) == 0x03);

    ++it;
    REQUIRE(it != end);
    REQUIRE(std::wcscmp(it->name.data(), L"Other") == 0);
    REQUIRE(it->guid->Data1 == 0xAAAAAAAA);
    REQUIRE(it->value.size() == 2);
    REQUIRE(static_cast<unsigned>(it->value[0]) == 0xAA);
    REQUIRE(static_cast<unsigned>(it->value[1]) == 0xBB);

    ++it;
    REQUIRE(it == end);
}

TEST_CASE("EfiVariableRange supports range-for", "[firmware]") {
    auto buf = BuildBuffer();
    EfiVariableRange range(buf.data(), buf.data() + buf.size());

    std::vector<std::wstring> names;
    for (auto&& v : range) {
        names.emplace_back(v.name);
    }
    REQUIRE(names.size() == 2);
    REQUIRE(std::wcscmp(names[0].c_str(), L"TestVar") == 0);
    REQUIRE(std::wcscmp(names[1].c_str(), L"Other") == 0);
}

TEST_CASE("Empty buffer produces no entries", "[firmware]") {
    std::vector<std::uint8_t> empty;
    EfiVariableRange range(empty.data(), empty.data());
    int count = 0;
    for (auto&& v : range) {
        (void)v;
        ++count;
    }
    REQUIRE(count == 0);
}

TEST_CASE("Buffer too small for header produces no entries", "[firmware]") {
    std::vector<std::uint8_t> tiny(16, 0);
    EfiVariableRange range(tiny.data(), tiny.data() + tiny.size());
    int count = 0;
    for (auto&& v : range) {
        (void)v;
        ++count;
    }
    REQUIRE(count == 0);
}

TEST_CASE("Default-constructed iterators compare equal", "[firmware]") {
    EfiVariableIterator a;
    EfiVariableIterator b;
    REQUIRE(a == b);
}

TEST_CASE("EfiNameIterator iterates two entries", "[firmware]") {
    auto buf = BuildNameBuffer();
    EfiNameRange range(buf.data(), buf.data() + buf.size());

    auto it = range.begin();
    auto end = range.end();

    REQUIRE(it != end);
    REQUIRE(std::wcscmp(it->name.data(), L"TestVar") == 0);
    REQUIRE(it->name.size() == 8); // includes terminating null
    REQUIRE(it->guid->Data1 == 0x11111111);
    REQUIRE(it->guid->Data2 == 0x2222);
    REQUIRE(it->guid->Data3 == 0x3333);

    ++it;
    REQUIRE(it != end);
    REQUIRE(std::wcscmp(it->name.data(), L"Other") == 0);
    REQUIRE(it->name.size() == 6); // includes terminating null
    REQUIRE(it->guid->Data1 == 0xAAAAAAAA);

    ++it;
    REQUIRE(it == end);
}

TEST_CASE("EfiNameRange supports range-for", "[firmware]") {
    auto buf = BuildNameBuffer();
    EfiNameRange range(buf.data(), buf.data() + buf.size());

    std::vector<std::wstring> names;
    for (auto&& v : range) {
        names.emplace_back(v.name);
    }
    REQUIRE(names.size() == 2);
    REQUIRE(std::wcscmp(names[0].c_str(), L"TestVar") == 0);
    REQUIRE(std::wcscmp(names[1].c_str(), L"Other") == 0);
}

TEST_CASE("EfiNameIterator handles empty buffer", "[firmware]") {
    std::vector<std::uint8_t> empty;
    EfiNameRange range(empty.data(), empty.data());
    int count = 0;
    for (auto&& v : range) {
        (void)v;
        ++count;
    }
    REQUIRE(count == 0);
}

TEST_CASE("EfiNameIterator handles buffer too small for header", "[firmware]") {
    std::vector<std::uint8_t> tiny(16, 0);
    EfiNameRange range(tiny.data(), tiny.data() + tiny.size());
    int count = 0;
    for (auto&& v : range) {
        (void)v;
        ++count;
    }
    REQUIRE(count == 0);
}

TEST_CASE("Default-constructed EfiNameIterator compares equal", "[firmware]") {
    EfiNameIterator a;
    EfiNameIterator b;
    REQUIRE(a == b);
}

TEST_CASE("Read fails with invalid GUID string", "[firmware]") {
    auto result = efivar::service::Firmware::Read(L"TestVar", L"not-a-guid");
    REQUIRE(!result);
}

#include "token.hpp"

TEST_CASE("Read finds UbuntuToken firmware variable", "[firmware][integration]") {
    auto privilege = efivar::service::TokenPrivilege::Acquire(SE_SYSTEM_ENVIRONMENT_NAME);
    if (!privilege) {
        SKIP("SeSystemEnvironmentPrivilege not available — not running elevated");
    }

    constexpr const wchar_t* kTargetName = L"UbuntuToken";
    constexpr const wchar_t* kTargetGuidStr = L"{4f72e91a-a5b3-4c9d-8a6e-23d57bf4e9ac}";

    auto result = efivar::service::Firmware::Read(kTargetName, kTargetGuidStr);
    REQUIRE(result);
}

TEST_CASE("Enumeration finds UbuntuToken firmware variable", "[firmware][integration]") {
    auto privilege = efivar::service::TokenPrivilege::Acquire(SE_SYSTEM_ENVIRONMENT_NAME);
    if (!privilege) {
        SKIP("SeSystemEnvironmentPrivilege not available — not running elevated");
    }

    constexpr const wchar_t* kTargetName = L"UbuntuToken";
    constexpr const wchar_t* kTargetGuidStr = L"{4f72e91a-a5b3-4c9d-8a6e-23d57bf4e9ac}";

    GUID targetGuid{};
    REQUIRE(SUCCEEDED(CLSIDFromString(kTargetGuidStr, &targetGuid)));

    ULONG length = 0;
    LONG status = efivar::service::Firmware::NtEnumerateSystemEnvironmentValuesEx(
        efivar::service::Firmware::SystemEnvironmentNameInformation, nullptr, &length);
    REQUIRE(status == efivar::service::Firmware::StatusBufferTooSmall);
    REQUIRE(length > 0);

    std::vector<std::uint8_t> raw(length);
    status = efivar::service::Firmware::NtEnumerateSystemEnvironmentValuesEx(
        efivar::service::Firmware::SystemEnvironmentNameInformation, raw.data(), &length);
    REQUIRE(status == efivar::service::Firmware::StatusSuccess);

    efivar::service::Firmware::EfiNameRange range(raw.data(), raw.data() + length);
    bool found = false;
    for (auto&& v : range) {
        if (v.name == kTargetName &&
            std::memcmp(v.guid, &targetGuid, sizeof(GUID)) == 0) {
            found = true;
            break;
        }
    }
    REQUIRE(found);
}
