// dllmain.cpp : Defines the entry point for the DLL application.
#include "framework.hpp"

#ifdef _MSC_VER
#include <Windows.h>

#include <iostream>
#include <string_view>

int DebugReportHook(int reportType, char* message, int* returnValue) {
  const auto type = [=]() -> std::string_view {
    switch (reportType) {
      case _CRT_WARN:
        return "[WARNING]";
      case _CRT_ERROR:
        return "[ERROR]";
      case _CRT_ASSERT:
        return "[ASSERT]";
      default:
        return "[UNKNOWN]";
    }
  }();

  std::cerr << type << ' ' << message << std::endl;
  throw std::runtime_error(message);
}
#else
#define _CrtSetReportHook(x)
#endif  // _MSC_VER

BOOL APIENTRY DllMain(HMODULE hModule, DWORD ul_reason_for_call,
                      LPVOID lpReserved) {
  _CrtSetReportHook(DebugReportHook);

  switch (ul_reason_for_call) {
    case DLL_PROCESS_ATTACH:
    case DLL_THREAD_ATTACH:
    case DLL_THREAD_DETACH:
    case DLL_PROCESS_DETACH:
      break;
  }
  return TRUE;
}
