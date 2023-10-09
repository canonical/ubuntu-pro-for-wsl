#if defined UP4W_TEST_WITH_MS_STORE_MOCK && defined _MSC_VER

#include "WinMockContext.hpp"

#include <Windows.h>
#include <processenv.h>
#include <winrt/Windows.Data.Json.h>
#include <winrt/Windows.Foundation.Collections.h>
#include <winrt/Windows.Foundation.h>
#include <winrt/Windows.Web.Http.Filters.h>
#include <winrt/Windows.Web.Http.h>
#include <winrt/base.h>

#include <algorithm>
#include <cassert>
#include <functional>
#include <iterator>
#include <sstream>
#include <unordered_map>
#include <utility>

#include "WinRTHelpers.hpp"

namespace StoreApi::impl {

using winrt::Windows::Data::Json::IJsonValue;
using winrt::Windows::Data::Json::JsonArray;
using winrt::Windows::Data::Json::JsonObject;
using UrlParams = std::unordered_multimap<winrt::hstring, winrt::hstring>;
using winrt::Windows::Foundation::Uri;

using winrt::Windows::Foundation::IAsyncOperation;

namespace {
// Handles the HTTP calls, returning a JsonObject containing the mock server
// response. Notice that the supplied path is relative.
IAsyncOperation<JsonObject> call(winrt::hstring relativePath,
                                 UrlParams const& params = {});

/// Translates a textual representation of a purchase transaction result into an
/// instance of the PurchaseStatus enum.
StoreApi::PurchaseStatus translate(winrt::hstring const& purchaseStatus);
}  // namespace

std::vector<WinMockContext::Product> WinMockContext::GetProducts(
    std::span<const std::string> kinds,
    std::span<const std::string> ids) const {
  assert(!kinds.empty() && "kinds vector cannot be empty");
  assert(!ids.empty() && "ids vector cannot be empty");

  auto hKinds = to_hstrings(kinds);
  auto hIds = to_hstrings(ids);

  UrlParams parameters;

  std::ranges::transform(
      hKinds, std::inserter(parameters, parameters.end()),
      [](winrt::hstring k) { return std::make_pair(L"kinds", k); });
  std::ranges::transform(
      hIds, std::inserter(parameters, parameters.end()),
      [](winrt::hstring id) { return std::make_pair(L"ids", id); });

  auto productsJson = call(L"/products", parameters).get();

  JsonArray products = productsJson.GetNamedArray(L"products");

  std::vector<WinMockContext::Product> result;
  result.reserve(products.Size());
  for (const IJsonValue& product : products) {
    JsonObject p = product.GetObject();
    result.emplace_back(WinMockContext::Product{p});
  }

  return result;
}

std::vector<std::string> WinMockContext::AllLocallyAuthenticatedUserHashes() {
  JsonObject usersList = call(L"/allauthenticatedusers").get();
  JsonArray users = usersList.GetNamedArray(L"users");

  std::vector<std::string> result;
  result.reserve(users.Size());
  for (const IJsonValue& user : users) {
    result.emplace_back(winrt::to_string(user.GetString()));
  }

  return result;
}

std::string WinMockContext::GenerateUserJwt(std::string token,
                                            std::string userId) const {
  assert(!token.empty() && "Azure AD token is required");
  JsonObject res{nullptr};

  UrlParams parameters{
      {L"serviceticket", winrt::to_hstring(token)},
  };
  if (!userId.empty()) {
    parameters.insert({L"publisheruserid", winrt::to_hstring(userId)});
  }

  res = call(L"generateuserjwt", parameters).get();

  return winrt::to_string(res.GetNamedString(L"jwt"));
}

// NOOP, for now at least.
void WinMockContext::InitDialogs(Window parentWindow) {}

void WinMockContext::Product::PromptUserForPurchase(
    PurchaseCallback callback) const {
  using winrt::Windows::Foundation::AsyncStatus;

  UrlParams params{{L"id", winrt::to_hstring(storeID)}};
  call(L"purchase", params)
      .Completed(
          [cb = std::move(callback)](IAsyncOperation<JsonObject> const& async,
                                     AsyncStatus const& asyncStatus) {
            PurchaseStatus translated;
            std::int32_t error = async.ErrorCode().value;
            if (error != 0 || asyncStatus == AsyncStatus::Error) {
              translated = PurchaseStatus::NetworkError;
            } else {
              auto json = async.GetResults();
              auto status = json.GetNamedString(L"status");
              translated = translate(status);
            }

            cb(translated, error);
          });
}

WinMockContext::Product::Product(JsonObject const& json)
    : storeID{winrt::to_string(json.GetNamedString(L"StoreID"))},
      title{winrt::to_string(json.GetNamedString(L"Title"))},
      description{winrt::to_string(json.GetNamedString(L"Description"))},
      productKind{winrt::to_string(json.GetNamedString(L"ProductKind"))},
      expirationDate{},
      isInUserCollection{json.GetNamedBoolean(L"IsInUserCollection")}

{
  std::chrono::system_clock::time_point tp{};
  std::stringstream ss{
      winrt::to_string(json.GetNamedString(L"ExpirationDate"))};
  ss >> std::chrono::parse("%FT%T%Tz", tp);
  expirationDate = tp;
}

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
  namespace http = winrt::Windows::Web::Http;
  static http::Filters::HttpBaseProtocolFilter filter{};
  filter.CacheControl().ReadBehavior(
      http::Filters::HttpCacheReadBehavior::NoCache);
  filter.CacheControl().WriteBehavior(
      http::Filters::HttpCacheWriteBehavior::NoCache);
  static http::HttpClient httpClient{filter};

  Uri uri = co_await buildUri(relativePath, params);

  // We can rely on the fact that our mock will return small pieces of data
  // certainly under 1 KB.
  winrt::hstring contents = co_await httpClient.GetStringAsync(uri);
  co_return JsonObject::Parse(contents);
}

StoreApi::PurchaseStatus translate(winrt::hstring const& purchaseStatus) {
  if (purchaseStatus == L"Succeeded") {
    return PurchaseStatus::Succeeded;
  }

  if (purchaseStatus == L"AlreadyPurchased") {
    return StoreApi::PurchaseStatus::AlreadyPurchased;
  }

  if (purchaseStatus == L"NotPurchased") {
    return StoreApi::PurchaseStatus::UserGaveUp;
  }

  if (purchaseStatus == L"ServerError") {
    return StoreApi::PurchaseStatus::ServerError;
  }

  assert(false && "Missing enum elements to translate StorePurchaseStatus.");
  return StoreApi::PurchaseStatus::Unknown;  // To be future proof.
}

}  // namespace
}  // namespace StoreApi::impl
#endif  // UP4W_TEST_WITH_MS_STORE_MOCK
