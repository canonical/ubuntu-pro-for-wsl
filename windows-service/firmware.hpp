#pragma once

#include "utility.hpp"

#include <cstring>
#include <expected>
#include <iterator>
#include <span>
#include <string>
#include <string_view>
#include <vector>
#include <objbase.h>
#include <guiddef.h>
#include <windows.h>

namespace efivar::service::Firmware {

constexpr ULONG SystemEnvironmentNameInformation = 1;
constexpr ULONG SystemEnvironmentValuesInformation = 2;
constexpr LONG StatusBufferTooSmall = 0xC0000023;
constexpr LONG StatusSuccess = 0;
constexpr LONG ReadZeroEntries = 1;

extern "C" __declspec(dllimport) LONG NTAPI NtEnumerateSystemEnvironmentValuesEx(
    ULONG InformationClass,
    PVOID Buffer,
    PULONG BufferLength);

struct ReadResult {
    std::string buffer;
    DWORD bytesRead = 0;
};

inline std::expected<ReadResult, std::error_code> Find(
    const wchar_t* name,
    const GUID& vendorGuid);

// --- EfiVariableView and range adaptor over VARIABLE_NAME_AND_VALUE linked list ---

// VARIABLE_NAME_AND_VALUE header layout
namespace detail {
constexpr size_t kNextEntryOffset = 0;
constexpr size_t kValueOffset     = sizeof(ULONG);
constexpr size_t kValueLengthOffset = sizeof(ULONG) * 2;
constexpr size_t kVendorGuid      = sizeof(ULONG) * 4;
constexpr size_t kName            = kVendorGuid + sizeof(GUID);
constexpr size_t kHeaderSize      = kName;
}

struct EfiVariableView {
    const GUID* guid;
    std::wstring_view name;
    std::span<const std::byte> value;
};

class EfiVariableIterator {
public:
    using iterator_category = std::input_iterator_tag;
    using value_type = EfiVariableView;
    using difference_type = std::ptrdiff_t;
    using pointer = const EfiVariableView*;
    using reference = const EfiVariableView&;

    EfiVariableIterator() = default;

    EfiVariableIterator(const std::uint8_t* entry, const std::uint8_t* end)
        : current_(entry), end_(end) {
        if (current_) {
            load_view();
        }
    }

    reference operator*() const { return view_; }
    pointer operator->() const { return &view_; }

    EfiVariableIterator& operator++() {
        if (nextOffset_ == 0 || current_ + nextOffset_ >= end_) {
            current_ = end_;
        } else {
            current_ += nextOffset_;
            load_view();
        }
        return *this;
    }

    EfiVariableIterator operator++(int) {
        auto copy = *this;
        ++(*this);
        return copy;
    }

    bool operator==(const EfiVariableIterator& other) const {
        return current_ == other.current_;
    }

private:
    void load_view() {
        using namespace detail;
        if (current_ + kHeaderSize > end_) {
            current_ = end_;
            return;
        }
        
        nextOffset_ = *reinterpret_cast<const ULONG*>(current_ + kNextEntryOffset);
        ULONG valueOffset = *reinterpret_cast<const ULONG*>(current_ + kValueOffset);
        ULONG valueLength = *reinterpret_cast<const ULONG*>(current_ + kValueLengthOffset);

        view_.guid = reinterpret_cast<const GUID*>(current_ + kVendorGuid);
        view_.value = std::span<const std::byte>(reinterpret_cast<const std::byte*>(current_ + valueOffset), valueLength);

        const wchar_t* nameStart = reinterpret_cast<const wchar_t*>(current_ + kName);
        size_t nameByteLen = valueOffset - kName;
        size_t nameLen = nameByteLen / sizeof(wchar_t);
        view_.name = std::wstring_view(nameStart, nameLen);
    }

    const std::uint8_t* current_ = nullptr;
    const std::uint8_t* end_ = nullptr;
    ULONG nextOffset_ = 0;
    EfiVariableView view_{};
};

class EfiVariableRange {
public:
    EfiVariableRange(const std::uint8_t* begin, const std::uint8_t* end)
        : begin_(begin), end_(end) {}

    EfiVariableIterator begin() const { return EfiVariableIterator(begin_, end_); }
    EfiVariableIterator end() const { return EfiVariableIterator(end_, end_); }

private:
    const std::uint8_t* begin_;
    const std::uint8_t* end_;
};

// --- EfiNameView and range adaptor over VARIABLE_NAME linked list ---

// VARIABLE_NAME header layout
namespace detail {
constexpr size_t kNameNextEntryOffset = 0;
constexpr size_t kNameVendorGuid      = sizeof(ULONG);
constexpr size_t kNameHeaderSize      = sizeof(ULONG) + sizeof(GUID);
}

struct EfiNameView {
    const GUID* guid;
    std::wstring_view name;
};

class EfiNameIterator {
public:
    using iterator_category = std::input_iterator_tag;
    using value_type = EfiNameView;
    using difference_type = std::ptrdiff_t;
    using pointer = const EfiNameView*;
    using reference = const EfiNameView&;

    EfiNameIterator() = default;

    EfiNameIterator(const std::uint8_t* entry, const std::uint8_t* end)
        : current_(entry), end_(end) {
        if (current_) {
            load_view();
        }
    }

    reference operator*() const { return view_; }
    pointer operator->() const { return &view_; }

    EfiNameIterator& operator++() {
        if (nextOffset_ == 0 || current_ + nextOffset_ >= end_) {
            current_ = end_;
        } else {
            current_ += nextOffset_;
            load_view();
        }
        return *this;
    }

    EfiNameIterator operator++(int) {
        auto copy = *this;
        ++(*this);
        return copy;
    }

    bool operator==(const EfiNameIterator& other) const {
        return current_ == other.current_;
    }

private:
    void load_view() {
        using namespace detail;
        if (current_ + kNameHeaderSize > end_) {
            current_ = end_;
            return;
        }

        nextOffset_ = *reinterpret_cast<const ULONG*>(current_ + kNameNextEntryOffset);
        view_.guid = reinterpret_cast<const GUID*>(current_ + kNameVendorGuid);

        size_t nameByteLen = 0;
        if (nextOffset_ != 0) {
            nameByteLen = nextOffset_ - kNameHeaderSize;
        } else {
            nameByteLen = static_cast<size_t>(end_ - current_ - kNameHeaderSize);
        }
        size_t nameLen = nameByteLen / sizeof(wchar_t);
        const wchar_t* nameStart = reinterpret_cast<const wchar_t*>(current_ + kNameHeaderSize);
        view_.name = std::wstring_view(nameStart, nameLen);
    }

    const std::uint8_t* current_ = nullptr;
    const std::uint8_t* end_ = nullptr;
    ULONG nextOffset_ = 0;
    EfiNameView view_{};
};

class EfiNameRange {
public:
    EfiNameRange(const std::uint8_t* begin, const std::uint8_t* end)
        : begin_(begin), end_(end) {}

    EfiNameIterator begin() const { return EfiNameIterator(begin_, end_); }
    EfiNameIterator end() const { return EfiNameIterator(end_, end_); }

private:
    const std::uint8_t* begin_;
    const std::uint8_t* end_;
};

inline std::expected<ReadResult, std::error_code> Read(
    const wchar_t* name,
    const wchar_t* guid) {

    GUID vendorGuid{};
    HRESULT hr = CLSIDFromString(guid, &vendorGuid);
    if (FAILED(hr)) {
        return std::unexpected(std::error_code(hr, std::system_category()));
    }

    ReadResult result;
    result.buffer.resize(1024, '\0');
    DWORD length = static_cast<DWORD>(result.buffer.size());
    DWORD bytesRead = GetFirmwareEnvironmentVariableW(
        name,
        guid,
        result.buffer.data(),
        length);

    if (bytesRead > 0) {
        result.bytesRead = bytesRead;
        return result;
    }

    DWORD err = GetLastError();
    if (err == ERROR_ENVVAR_NOT_FOUND) {
        return Find(name, vendorGuid);
    }

    return std::unexpected(std::error_code(static_cast<int>(err), std::system_category()));
}

inline std::expected<ReadResult, std::error_code> Find(
    const wchar_t* name,
    const GUID& vendorGuid) {

    ULONG length = 0;
    LONG status = NtEnumerateSystemEnvironmentValuesEx(
        SystemEnvironmentValuesInformation,
        nullptr,
        &length);

    if (status != StatusBufferTooSmall) {
        return std::unexpected(std::error_code(status, std::system_category()));
    }

    if (length == 0) {
        return std::unexpected(std::error_code(ERROR_SOURCE_ELEMENT_EMPTY, std::system_category()));
    }

    std::vector<std::uint8_t> raw(length);
    status = NtEnumerateSystemEnvironmentValuesEx(
        SystemEnvironmentValuesInformation,
        raw.data(),
        &length);

    if (status != StatusSuccess) {
        return std::unexpected(std::error_code(status, std::system_category()));
    }

    EfiVariableRange range(raw.data(), raw.data() + length);
    uint32_t count = 0;
    for (auto&& v : range) {
        count++;
        if (v.name == name && std::memcmp(v.guid, &vendorGuid, sizeof(GUID)) == 0 ) {
            ReadResult result;
            if (!v.value.empty()) {
                result.buffer.assign(
                    reinterpret_cast<const char*>(v.value.data()),
                    v.value.size());
                result.bytesRead = static_cast<DWORD>(v.value.size());
            }
            return result;
        }
    }

    return std::unexpected(std::error_code(ERROR_NO_MATCH, std::system_category()));
}

struct EnumerateResult {
    std::string buffer;
    ULONG bytesRead = 0;
};

inline std::expected<EnumerateResult, std::error_code> Enumerate() {
    EnumerateResult result;
    ULONG length = 0;
    LONG status = NtEnumerateSystemEnvironmentValuesEx(
        SystemEnvironmentNameInformation,
        nullptr,
        &length);

    if (status != StatusBufferTooSmall) {
        return std::unexpected(std::error_code(status, std::system_category()));
    }

    if (length == 0) {
        return result;
    }

    std::vector<std::uint8_t> raw(length);
    status = NtEnumerateSystemEnvironmentValuesEx(
        SystemEnvironmentNameInformation,
        raw.data(),
        &length);

    if (status != StatusSuccess) {
        return std::unexpected(std::error_code(status, std::system_category()));
    }

    EfiNameRange range(raw.data(), raw.data() + length);
    for (auto&& v : range) {
        wchar_t guidStr[39];
        StringFromGUID2(*v.guid, guidStr, 39);

        int nameUtf8Size = WideCharToMultiByte(CP_UTF8, 0, v.name.data(), static_cast<int>(v.name.size()), nullptr, 0, nullptr, nullptr);
        int guidUtf8Size = WideCharToMultiByte(CP_UTF8, 0, guidStr, 38, nullptr, 0, nullptr, nullptr);

        if (nameUtf8Size > 0 && guidUtf8Size > 0) {
            size_t oldSize = result.buffer.size();
            result.buffer.resize(oldSize + static_cast<size_t>(nameUtf8Size) + 1 + static_cast<size_t>(guidUtf8Size) + 1);
            char* out = result.buffer.data() + oldSize;
            WideCharToMultiByte(CP_UTF8, 0, v.name.data(), static_cast<int>(v.name.size()), out, nameUtf8Size, nullptr, nullptr);
            out += nameUtf8Size;
            *out++ = ' ';
            WideCharToMultiByte(CP_UTF8, 0, guidStr, 38, out, guidUtf8Size, nullptr, nullptr);
            out += guidUtf8Size;
            *out++ = '\n';
        }
    }

    result.bytesRead = static_cast<ULONG>(result.buffer.size());
    return result;
}

} // namespace efivar::service::Firmware
