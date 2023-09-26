// dllmain.cpp : Defines the entry point for the DLL application.
#include <stdexcept>

#include "framework.hpp"

#ifdef _MSC_VER
#include <Windows.h>
#else
constexpr void _CrtSetReportHook(auto) {}
#endif  // _MSC_VER

BOOL APIENTRY DllMain(HMODULE hModule, DWORD ul_reason_for_call,
                      LPVOID lpReserved) {
  _CrtSetReportHook([](int reportType, char* message, int* returnValue) -> int {
    throw std::runtime_error(message);
  });

  switch (ul_reason_for_call) {
    case DLL_PROCESS_ATTACH:
    case DLL_THREAD_ATTACH:
    case DLL_THREAD_DETACH:
    case DLL_PROCESS_DETACH:
      break;
  }
  return TRUE;
}
