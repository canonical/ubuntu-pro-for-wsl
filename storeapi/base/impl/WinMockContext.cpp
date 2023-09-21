#if defined UP4W_TEST_WITH_MS_STORE_MOCK && defined _MSC_VER

#include "WinMockContext.hpp"

#include <Windows.h>
#include <processenv.h>
#include <winrt/Windows.Data.Json.h>
#include <winrt/Windows.Foundation.Collections.h>
#include <winrt/Windows.Foundation.h>
#include <winrt/Windows.Web.Http.h>
#include <winrt/base.h>

#include <algorithm>
#include <sstream>
#include <unordered_map>

namespace StoreApi::impl {

using winrt::Windows::Data::Json::JsonObject;
using UrlParams = std::unordered_multimap<winrt::hstring, winrt::hstring>;
using winrt::Windows::Foundation::Uri;

using winrt::Windows::Foundation::IAsyncOperation;

namespace {
// Handles the HTTP calls, returning a JsonObject containing the mock server
// response. Notice that the supplied path is relative.
IAsyncOperation<JsonObject> call(winrt::hstring relativePath,
                                 UrlParams const& params = {});

/// Creates a product from a JsonObject containing the relevant information.
WinMockContext::Product fromJson(JsonObject const& obj);

}  // namespace

namespace {
// Returns the mock server endpoint address and port by reading the environment
// variable UP4W_MS_STORE_MOCK_ENDPOINT or localhost:9 if the variable is unset.
winrt::hstring readStoreMockEndpoint() {
  constexpr std::size_t endpointSize = 20;
  wchar_t endpoint[endpointSize];
  if (0 == GetEnvironmentVariableW(L"UP4W_MS_STORE_MOCK_ENDPOINT", endpoint,
                                   endpointSize)) {
    return L"127.0.0.1:9";  // Discard protocol
  }
  return endpoint;
}

// Builds a complete URI with a URL encoded query if params are passed.
IAsyncOperation<Uri> buildUri(winrt::hstring& relativePath,
                              UrlParams const& params) {
  // Being tied t an environment variable means that it cannot change after
  // program's creation. Thus, there is no reason for recreating this value
  // every call.
  static winrt::hstring endpoint = L"http://" + readStoreMockEndpoint();

  if (!params.empty()) {
    winrt::Windows::Web::Http::HttpFormUrlEncodedContent p{
        {params.begin(), params.end()},
    };
    auto rawParams = co_await p.ReadAsStringAsync();
    // http://127.0.0.1:56567/relativePath?param=value...
    co_return Uri{endpoint, relativePath + L'?' + rawParams};
  }
  // http://127.0.0.1:56567/relativePath
  co_return Uri{endpoint, relativePath};
}

IAsyncOperation<JsonObject> call(winrt::hstring relativePath,
                                 UrlParams const& params) {
  // Initialize only once.
  static winrt::Windows::Web::Http::HttpClient httpClient{};

  Uri uri = co_await buildUri(relativePath, params);

  // We can rely on the fact that our mock will return small pieces of data
  // certainly under 1 KB.
  winrt::hstring contents = co_await httpClient.GetStringAsync(uri);
  co_return JsonObject::Parse(contents);
}

WinMockContext::Product fromJson(JsonObject const& obj) {
  std::chrono::system_clock::time_point tp{};
  std::stringstream ss{winrt::to_string(obj.GetNamedString(L"ExpirationDate"))};
  ss >> std::chrono::parse("%FT%T%Tz", tp);

  return WinMockContext::Product{
      winrt::to_string(obj.GetNamedString(L"StoreID")),
      winrt::to_string(obj.GetNamedString(L"Title")),
      winrt::to_string(obj.GetNamedString(L"Description")),
      winrt::to_string(obj.GetNamedString(L"ProductKind")),
      tp,
      obj.GetNamedBoolean(L"IsInUserCollection")};
}

}  // namespace
}  // namespace StoreApi::impl
#endif  // UP4W_TEST_WITH_MS_STORE_MOCK
