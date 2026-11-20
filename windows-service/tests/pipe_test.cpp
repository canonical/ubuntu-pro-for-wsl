#include "pipe.hpp"
#include "protocol.h"
#include "security.hpp"

#include <catch2/catch_test_macros.hpp>

#include <thread>
#include <windows.h>

using efivar::service::Pipe;
using efivar::service::SecurityDescriptor;

struct TestMessage {
    DWORD value;
};

constexpr wchar_t TestPipeName[] = L"\\\\.\\pipe\\LOCAL\\efivar-service-test";

TEST_CASE("Pipe Create succeeds with security descriptor", "[pipe]") {
    auto sd = SecurityDescriptor::Create();
    REQUIRE(sd.has_value());

    auto pipe = Pipe::Create(TestPipeName, sd->get());
    INFO("Pipe create error: " << (pipe ? 0 : pipe.error().value()));
    REQUIRE(pipe.has_value());
}

TEST_CASE("Pipe Accept and Connection Read/Write round-trip", "[pipe]") {
    auto sd = SecurityDescriptor::Create();
    REQUIRE(sd.has_value());

    auto pipe = Pipe::Create(TestPipeName, sd->get());
    INFO("Pipe create error: " << (pipe ? 0 : pipe.error().value()));
    REQUIRE(pipe.has_value());

    wil::unique_event stopEvent(CreateEventW(nullptr, TRUE, FALSE, nullptr));
    REQUIRE(stopEvent.is_valid());

    std::thread client([]() {
        // Give the server a moment to start accepting.
        Sleep(50);
        wil::unique_handle handle(CreateFileW(
            TestPipeName,
            GENERIC_READ | GENERIC_WRITE,
            0,
            nullptr,
            OPEN_EXISTING,
            0,
            nullptr));
        INFO("Client connect error: " << GetLastError());
        REQUIRE(handle.is_valid());

        TestMessage sent{42};
        DWORD written = 0;
        REQUIRE(WriteFile(handle.get(), &sent, sizeof(sent), &written, nullptr));
        REQUIRE(written == sizeof(sent));

        TestMessage received{};
        DWORD read = 0;
        REQUIRE(ReadFile(handle.get(), &received, sizeof(received), &read, nullptr));
        REQUIRE(read == sizeof(received));
        REQUIRE(received.value == 123);
    });

    auto connResult = pipe->Accept(stopEvent.get());
    INFO("Accept error: " << (connResult ? 0 : connResult.error().value()));
    REQUIRE(connResult.has_value());
    auto& conn = *connResult;

    auto readResult = conn.Read<TestMessage>();
    INFO("Read error: " << (readResult ? 0 : readResult.error().value()));
    REQUIRE(readResult.has_value());
    REQUIRE(readResult->value == 42);

    TestMessage reply{123};
    auto writeResult = conn.Write(&reply, sizeof(reply));
    INFO("Write error: " << (writeResult ? 0 : writeResult.error().value()));
    REQUIRE(writeResult.has_value());

    client.join();
}
