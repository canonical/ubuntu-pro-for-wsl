#pragma once

#include "utility.hpp"

#include <expected>
#include <windows.h>
#include <wil/resource.h>

namespace efivar::service {

class Pipe {
public:
    class Connection {
        HANDLE pipeHandle_ = INVALID_HANDLE_VALUE;
        wil::unique_event event_;
        bool connected_ = false;

        Connection(HANDLE pipe, wil::unique_event event, bool connected)
            : pipeHandle_(pipe), event_(std::move(event)), connected_(connected) {}

        friend class Pipe;

    public:
        Connection() = default;
        ~Connection() {
            if (connected_ && pipeHandle_ != INVALID_HANDLE_VALUE) {
                FlushFileBuffers(pipeHandle_);
                DisconnectNamedPipe(pipeHandle_);
            }
        }

        Connection(const Connection&) = delete;
        Connection& operator=(const Connection&) = delete;

        Connection(Connection&& other) noexcept
            : pipeHandle_(other.pipeHandle_),
              event_(std::move(other.event_)),
              connected_(other.connected_) {
            other.connected_ = false;
            other.pipeHandle_ = INVALID_HANDLE_VALUE;
        }

        Connection& operator=(Connection&& other) noexcept {
            if (this != &other) {
                pipeHandle_ = other.pipeHandle_;
                event_ = std::move(other.event_);
                connected_ = other.connected_;
                other.connected_ = false;
                other.pipeHandle_ = INVALID_HANDLE_VALUE;
            }
            return *this;
        }

        template<typename T>
        std::expected<T, std::error_code> Read() {
            T value{};
            DWORD read = 0;
            if (!ReadFile(pipeHandle_, &value, sizeof(T), &read, nullptr)) {
                return std::unexpected(last_error());
            }
            if (read != sizeof(T)) {
                return std::unexpected(last_error(ERROR_HANDLE_EOF));
            }
            return value;
        }

        std::expected<void, std::error_code> Write(const void* buf, DWORD len) {
            DWORD written = 0;
            if (!WriteFile(pipeHandle_, buf, len, &written, nullptr)) {
                return std::unexpected(last_error());
            }
            if (written != len) {
                return std::unexpected(last_error(ERROR_WRITE_FAULT));
            }
            return {};
        }
    };

private:
    wil::unique_handle handle_;

public:
    Pipe() = default;
    explicit Pipe(HANDLE handle) : handle_(handle) {}

    Pipe(const Pipe&) = delete;
    Pipe& operator=(const Pipe&) = delete;

    Pipe(Pipe&&) = default;
    Pipe& operator=(Pipe&&) = default;

    ~Pipe() {
        if (handle_.is_valid()) {
            CancelIoEx(handle_.get(), nullptr);
        }
    }

    static std::expected<Pipe, std::error_code> Create(
        const wchar_t* name,
        SECURITY_ATTRIBUTES* sa) {
        HANDLE handle = CreateNamedPipeW(
            name,
            PIPE_ACCESS_DUPLEX | FILE_FLAG_OVERLAPPED,
            PIPE_TYPE_MESSAGE | PIPE_READMODE_MESSAGE | PIPE_WAIT | PIPE_REJECT_REMOTE_CLIENTS,
            1,
            4096,
            4096,
            0,
            sa);

        if (handle == INVALID_HANDLE_VALUE) {
            return std::unexpected(last_error());
        }

        return Pipe(handle);
    }

    HANDLE get() const noexcept { return handle_.get(); }

    std::expected<Connection, std::error_code> Accept(HANDLE stopEvent) {
        wil::unique_event event(
            CreateEventW(nullptr, TRUE, FALSE, nullptr));
        if (!event.is_valid()) {
            return std::unexpected(last_error());
        }

        OVERLAPPED ol{};
        ol.hEvent = event.get();

        BOOL connected = ConnectNamedPipe(handle_.get(), &ol);
        DWORD err = GetLastError();

        if (!connected && err != ERROR_IO_PENDING && err != ERROR_PIPE_CONNECTED) {
            return std::unexpected(last_error(err));
        }

        if (!connected && err == ERROR_IO_PENDING) {
            HANDLE waitHandles[2] = { event.get(), stopEvent };
            DWORD waitResult = WaitForMultipleObjects(2, waitHandles, FALSE, INFINITE);

            if (waitResult == WAIT_OBJECT_0 + 1) {
                CancelIoEx(handle_.get(), &ol);
                return std::unexpected(last_error(ERROR_OPERATION_ABORTED));
            }

            if (waitResult != WAIT_OBJECT_0) {
                CancelIoEx(handle_.get(), &ol);
                return std::unexpected(last_error());
            }

            DWORD dummy = 0;
            if (!GetOverlappedResult(handle_.get(), &ol, &dummy, FALSE)) {
                return std::unexpected(last_error());
            }
        }

        return Connection(handle_.get(), std::move(event), true);
    }
};

} // namespace efivar::service
