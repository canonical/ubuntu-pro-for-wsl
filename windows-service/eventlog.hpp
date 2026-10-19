#pragma once

#include "utility.hpp"

#include <expected>
#include <string>
#include <windows.h>
#include <wil/resource.h>

namespace efivar::service {

class EventLog {
    wil::unique_handle source_;

    explicit EventLog(HANDLE source) : source_(source) {}

public:
    EventLog() = default;

    EventLog(const EventLog&) = delete;
    EventLog& operator=(const EventLog&) = delete;

    EventLog(EventLog&&) = default;
    EventLog& operator=(EventLog&&) = default;

    static std::expected<EventLog, std::error_code> Open(const wchar_t* sourceName) {
        HANDLE source = RegisterEventSourceW(nullptr, sourceName);
        if (!source) {
            return std::unexpected(last_error());
        }
        return EventLog(source);
    }

    void Info(DWORD eventId, const std::wstring& message) const {
        Report(EVENTLOG_INFORMATION_TYPE, eventId, message);
    }

    void Warning(DWORD eventId, const std::wstring& message) const {
        Report(EVENTLOG_WARNING_TYPE, eventId, message);
    }

    void Error(DWORD eventId, const std::wstring& message) const {
        Report(EVENTLOG_ERROR_TYPE, eventId, message);
    }

private:
    void Report(WORD eventType, DWORD eventId, const std::wstring& message) const {
        if (!source_.is_valid()) {
            return;
        }
        const wchar_t* strings[] = { message.c_str() };
        ReportEventW(source_.get(), eventType, 0, eventId, nullptr, 1, 0, strings, nullptr);
    }
};

} // namespace efivar::service
