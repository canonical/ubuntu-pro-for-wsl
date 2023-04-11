#include <flutter/method_result_functions.h>
#include <gtest/gtest.h>
#include <p4w_methods.h>

#include <memory>
#include <string>

#include "mocks.h"

namespace p4w_ms_store {

TEST(Methods, NotImplemented) {
  const flutter::MethodCall<flutter::EncodableValue> method_call(
      "surely-not-implemented", nullptr);
  bool notImplCalled = false;
  auto res =
      std::make_unique<flutter::MethodResultFunctions<flutter::EncodableValue>>(
          // on sucess
          [](const auto* _) {},  // don't care
          // on error
          [](const auto& _, const auto& __, const auto* ___) {
          },  // don't care either
          // on not implemented
          [&notImplCalled]() { notImplCalled = true; });

  Method m{method_call};

  m.call(std::move(res));

  EXPECT_TRUE(notImplCalled);
}

TEST(Launch, AcessDenied) {
  const std::string errorMsg{"Access denied"};
  const flutter::MethodCall<flutter::EncodableValue> method_call(
      ChannelUtil::method_name(ChannelUtil::launch), nullptr);
  bool onErrorCalled = false;
  bool msgMatched = false;

  auto result =
      std::make_unique<flutter::MethodResultFunctions<flutter::EncodableValue>>(
          // on sucess
          [](const auto* _) {},  // don't care
          // on error
          [&onErrorCalled, &msgMatched, &errorMsg](
              const auto& _, const auto& msg, const auto* ___) {
            onErrorCalled = true;
            msgMatched = (errorMsg == msg);
          },
          // on not implemented
          []() {});  // don't care either.
  StubApi::instance = std::make_unique<StubApi>();

  StubApi::instance->on_launch =
      [&errorMsg](
          std::unique_ptr<flutter::MethodResult<flutter::EncodableValue>> res)
      -> winrt::fire_and_forget {
    // mimics an API failure.
    res->Error("test-channel", errorMsg);
    return {};
  };

  Method m{method_call};
  m.call<StubApi>(std::move(result));

  EXPECT_TRUE(onErrorCalled);
  EXPECT_TRUE(msgMatched);
}

TEST(Launch, WrongArgs) {
  const flutter::MethodCall<flutter::EncodableValue> method_call(
      ChannelUtil::method_name(ChannelUtil::launch),
      std::make_unique<flutter::EncodableValue>(42));

  EXPECT_THROW({ Method m{method_call}; }, std::invalid_argument);
}

TEST(Launch, Success) {
  const flutter::MethodCall<flutter::EncodableValue> method_call(
      ChannelUtil::method_name(ChannelUtil::launch), nullptr);
  bool successCalled = false;
  auto result =
      std::make_unique<flutter::MethodResultFunctions<flutter::EncodableValue>>(
          // on sucess
          [&successCalled](const auto* _) { successCalled = true; },
          // on error
          [](const auto& _, const auto& __, const auto* ___) {},  // don't care
          // on not implemented
          []() {});  // don't care either.
  StubApi::instance = std::make_unique<StubApi>();

  StubApi::instance->on_launch =
      [](std::unique_ptr<flutter::MethodResult<flutter::EncodableValue>> res)
      -> winrt::fire_and_forget {
    // mimics an API failure.
    res->Success();
    return {};
  };

  Method m{method_call};
  m.call<StubApi>(std::move(result));

  EXPECT_TRUE(successCalled);
}

TEST(Launch, SuccessWithArgs) {
  const std::string Arguments{"--with=args --and more_args"};
  const flutter::MethodCall<flutter::EncodableValue> method_call(
      ChannelUtil::method_name(ChannelUtil::launch),
      std::make_unique<flutter::EncodableValue>(Arguments));
  bool successCalled = false;
  bool argumentsMatched = false;
  auto result =
      std::make_unique<flutter::MethodResultFunctions<flutter::EncodableValue>>(
          // on sucess
          [&successCalled](const auto* _) { successCalled = true; },
          // on error
          [](const auto& _, const auto& __, const auto* ___) {},  // don't care
          // on not implemented
          []() {});  // don't care either.
  StubApi::instance = std::make_unique<StubApi>();

  StubApi::instance->on_launch_with_args =
      [&argumentsMatched, &Arguments](
          std::string_view args,
          std::unique_ptr<flutter::MethodResult<flutter::EncodableValue>> res)
      -> winrt::fire_and_forget {
    // mimics an API failure.
    argumentsMatched = (args == Arguments);
    res->Success();
    return {};
  };

  Method m{method_call};
  m.call<StubApi>(std::move(result));

  EXPECT_TRUE(successCalled);
  EXPECT_TRUE(argumentsMatched);
}

}  // namespace p4w_ms_store