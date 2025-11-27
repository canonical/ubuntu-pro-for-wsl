// This is a generated file - do not edit.
//
// Generated from agentapi.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports
// ignore_for_file: unused_import

import 'dart:convert' as $convert;
import 'dart:core' as $core;
import 'dart:typed_data' as $typed_data;

@$core.Deprecated('Use emptyDescriptor instead')
const Empty$json = {
  '1': 'Empty',
};

/// Descriptor for `Empty`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List emptyDescriptor =
    $convert.base64Decode('CgVFbXB0eQ==');

@$core.Deprecated('Use proAttachInfoDescriptor instead')
const ProAttachInfo$json = {
  '1': 'ProAttachInfo',
  '2': [
    {'1': 'token', '3': 1, '4': 1, '5': 9, '10': 'token'},
  ],
};

/// Descriptor for `ProAttachInfo`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List proAttachInfoDescriptor = $convert
    .base64Decode('Cg1Qcm9BdHRhY2hJbmZvEhQKBXRva2VuGAEgASgJUgV0b2tlbg==');

@$core.Deprecated('Use landscapeConfigDescriptor instead')
const LandscapeConfig$json = {
  '1': 'LandscapeConfig',
  '2': [
    {'1': 'config', '3': 1, '4': 1, '5': 9, '10': 'config'},
  ],
};

/// Descriptor for `LandscapeConfig`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List landscapeConfigDescriptor = $convert
    .base64Decode('Cg9MYW5kc2NhcGVDb25maWcSFgoGY29uZmlnGAEgASgJUgZjb25maWc=');

@$core.Deprecated('Use subscriptionInfoDescriptor instead')
const SubscriptionInfo$json = {
  '1': 'SubscriptionInfo',
  '2': [
    {'1': 'productId', '3': 1, '4': 1, '5': 9, '10': 'productId'},
    {
      '1': 'none',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.agentapi.Empty',
      '9': 0,
      '10': 'none'
    },
    {
      '1': 'user',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.agentapi.Empty',
      '9': 0,
      '10': 'user'
    },
    {
      '1': 'organization',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.agentapi.Empty',
      '9': 0,
      '10': 'organization'
    },
    {
      '1': 'microsoftStore',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.agentapi.Empty',
      '9': 0,
      '10': 'microsoftStore'
    },
  ],
  '8': [
    {'1': 'subscriptionType'},
  ],
};

/// Descriptor for `SubscriptionInfo`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List subscriptionInfoDescriptor = $convert.base64Decode(
    'ChBTdWJzY3JpcHRpb25JbmZvEhwKCXByb2R1Y3RJZBgBIAEoCVIJcHJvZHVjdElkEiUKBG5vbm'
    'UYAiABKAsyDy5hZ2VudGFwaS5FbXB0eUgAUgRub25lEiUKBHVzZXIYAyABKAsyDy5hZ2VudGFw'
    'aS5FbXB0eUgAUgR1c2VyEjUKDG9yZ2FuaXphdGlvbhgEIAEoCzIPLmFnZW50YXBpLkVtcHR5SA'
    'BSDG9yZ2FuaXphdGlvbhI5Cg5taWNyb3NvZnRTdG9yZRgFIAEoCzIPLmFnZW50YXBpLkVtcHR5'
    'SABSDm1pY3Jvc29mdFN0b3JlQhIKEHN1YnNjcmlwdGlvblR5cGU=');

@$core.Deprecated('Use landscapeSourceDescriptor instead')
const LandscapeSource$json = {
  '1': 'LandscapeSource',
  '2': [
    {
      '1': 'none',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.agentapi.Empty',
      '9': 0,
      '10': 'none'
    },
    {
      '1': 'user',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.agentapi.Empty',
      '9': 0,
      '10': 'user'
    },
    {
      '1': 'organization',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.agentapi.Empty',
      '9': 0,
      '10': 'organization'
    },
  ],
  '8': [
    {'1': 'landscapeSourceType'},
  ],
};

/// Descriptor for `LandscapeSource`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List landscapeSourceDescriptor = $convert.base64Decode(
    'Cg9MYW5kc2NhcGVTb3VyY2USJQoEbm9uZRgBIAEoCzIPLmFnZW50YXBpLkVtcHR5SABSBG5vbm'
    'USJQoEdXNlchgCIAEoCzIPLmFnZW50YXBpLkVtcHR5SABSBHVzZXISNQoMb3JnYW5pemF0aW9u'
    'GAMgASgLMg8uYWdlbnRhcGkuRW1wdHlIAFIMb3JnYW5pemF0aW9uQhUKE2xhbmRzY2FwZVNvdX'
    'JjZVR5cGU=');

@$core.Deprecated('Use configSourcesDescriptor instead')
const ConfigSources$json = {
  '1': 'ConfigSources',
  '2': [
    {
      '1': 'proSubscription',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.agentapi.SubscriptionInfo',
      '10': 'proSubscription'
    },
    {
      '1': 'landscapeSource',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.agentapi.LandscapeSource',
      '10': 'landscapeSource'
    },
  ],
};

/// Descriptor for `ConfigSources`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List configSourcesDescriptor = $convert.base64Decode(
    'Cg1Db25maWdTb3VyY2VzEkQKD3Byb1N1YnNjcmlwdGlvbhgBIAEoCzIaLmFnZW50YXBpLlN1Yn'
    'NjcmlwdGlvbkluZm9SD3Byb1N1YnNjcmlwdGlvbhJDCg9sYW5kc2NhcGVTb3VyY2UYAiABKAsy'
    'GS5hZ2VudGFwaS5MYW5kc2NhcGVTb3VyY2VSD2xhbmRzY2FwZVNvdXJjZQ==');

@$core.Deprecated('Use distroInfoDescriptor instead')
const DistroInfo$json = {
  '1': 'DistroInfo',
  '2': [
    {'1': 'wsl_name', '3': 1, '4': 1, '5': 9, '10': 'wslName'},
    {'1': 'id', '3': 2, '4': 1, '5': 9, '10': 'id'},
    {'1': 'version_id', '3': 3, '4': 1, '5': 9, '10': 'versionId'},
    {'1': 'pretty_name', '3': 4, '4': 1, '5': 9, '10': 'prettyName'},
    {'1': 'pro_attached', '3': 5, '4': 1, '5': 8, '10': 'proAttached'},
    {'1': 'hostname', '3': 6, '4': 1, '5': 9, '10': 'hostname'},
  ],
};

/// Descriptor for `DistroInfo`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List distroInfoDescriptor = $convert.base64Decode(
    'CgpEaXN0cm9JbmZvEhkKCHdzbF9uYW1lGAEgASgJUgd3c2xOYW1lEg4KAmlkGAIgASgJUgJpZB'
    'IdCgp2ZXJzaW9uX2lkGAMgASgJUgl2ZXJzaW9uSWQSHwoLcHJldHR5X25hbWUYBCABKAlSCnBy'
    'ZXR0eU5hbWUSIQoMcHJvX2F0dGFjaGVkGAUgASgIUgtwcm9BdHRhY2hlZBIaCghob3N0bmFtZR'
    'gGIAEoCVIIaG9zdG5hbWU=');

@$core.Deprecated('Use proAttachCmdDescriptor instead')
const ProAttachCmd$json = {
  '1': 'ProAttachCmd',
  '2': [
    {'1': 'token', '3': 1, '4': 1, '5': 9, '10': 'token'},
  ],
};

/// Descriptor for `ProAttachCmd`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List proAttachCmdDescriptor =
    $convert.base64Decode('CgxQcm9BdHRhY2hDbWQSFAoFdG9rZW4YASABKAlSBXRva2Vu');

@$core.Deprecated('Use landscapeConfigCmdDescriptor instead')
const LandscapeConfigCmd$json = {
  '1': 'LandscapeConfigCmd',
  '2': [
    {'1': 'config', '3': 1, '4': 1, '5': 9, '10': 'config'},
  ],
};

/// Descriptor for `LandscapeConfigCmd`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List landscapeConfigCmdDescriptor =
    $convert.base64Decode(
        'ChJMYW5kc2NhcGVDb25maWdDbWQSFgoGY29uZmlnGAEgASgJUgZjb25maWc=');

@$core.Deprecated('Use mSGDescriptor instead')
const MSG$json = {
  '1': 'MSG',
  '2': [
    {'1': 'wsl_name', '3': 1, '4': 1, '5': 9, '9': 0, '10': 'wslName'},
    {'1': 'result', '3': 2, '4': 1, '5': 9, '9': 0, '10': 'result'},
  ],
  '8': [
    {'1': 'data'},
  ],
};

/// Descriptor for `MSG`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List mSGDescriptor = $convert.base64Decode(
    'CgNNU0cSGwoId3NsX25hbWUYASABKAlIAFIHd3NsTmFtZRIYCgZyZXN1bHQYAiABKAlIAFIGcm'
    'VzdWx0QgYKBGRhdGE=');
