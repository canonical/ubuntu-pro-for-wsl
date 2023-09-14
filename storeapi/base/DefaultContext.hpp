#pragma once

#ifdef TEST_WITH_MS_STORE_MOCK
// TODO: Handle the mock case
#ifdef _MSC_VER
// windows specific mocked impl (may use winrt)
#else   // _MSC_VER
// non-windows specific mocked impl (Linux friendly)
#endif  // _MSC_VER

#else  // TEST_WITH_MS_STORE_MOCK
#include "impl/StoreContext.hpp"
namespace StoreApi {
using DefaultContext = impl::StoreContext;
}
#endif  // TEST_WITH_MS_STORE_MOCK
