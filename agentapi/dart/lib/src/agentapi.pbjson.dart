///
//  Generated code. Do not modify.
//  source: agentapi.proto
//
// @dart = 2.12
// ignore_for_file: annotate_overrides,camel_case_types,constant_identifier_names,deprecated_member_use_from_same_package,directives_ordering,library_prefixes,non_constant_identifier_names,prefer_final_fields,return_of_invalid_type,unnecessary_const,unnecessary_import,unnecessary_this,unused_import,unused_shown_name

import 'dart:core' as $core;
import 'dart:convert' as $convert;
import 'dart:typed_data' as $typed_data;
@$core.Deprecated('Use emptyDescriptor instead')
const Empty$json = const {
  '1': 'Empty',
};

/// Descriptor for `Empty`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List emptyDescriptor = $convert.base64Decode('CgVFbXB0eQ==');
@$core.Deprecated('Use proAttachInfoDescriptor instead')
const ProAttachInfo$json = const {
  '1': 'ProAttachInfo',
  '2': const [
    const {'1': 'token', '3': 1, '4': 1, '5': 9, '10': 'token'},
  ],
};

/// Descriptor for `ProAttachInfo`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List proAttachInfoDescriptor = $convert.base64Decode('Cg1Qcm9BdHRhY2hJbmZvEhQKBXRva2VuGAEgASgJUgV0b2tlbg==');
@$core.Deprecated('Use distroInfoDescriptor instead')
const DistroInfo$json = const {
  '1': 'DistroInfo',
  '2': const [
    const {'1': 'wsl_name', '3': 1, '4': 1, '5': 9, '10': 'wslName'},
    const {'1': 'id', '3': 2, '4': 1, '5': 9, '10': 'id'},
    const {'1': 'version_id', '3': 3, '4': 1, '5': 9, '10': 'versionId'},
    const {'1': 'pretty_name', '3': 4, '4': 1, '5': 9, '10': 'prettyName'},
    const {'1': 'pro_attached', '3': 5, '4': 1, '5': 8, '10': 'proAttached'},
  ],
};

/// Descriptor for `DistroInfo`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List distroInfoDescriptor = $convert.base64Decode('CgpEaXN0cm9JbmZvEhkKCHdzbF9uYW1lGAEgASgJUgd3c2xOYW1lEg4KAmlkGAIgASgJUgJpZBIdCgp2ZXJzaW9uX2lkGAMgASgJUgl2ZXJzaW9uSWQSHwoLcHJldHR5X25hbWUYBCABKAlSCnByZXR0eU5hbWUSIQoMcHJvX2F0dGFjaGVkGAUgASgIUgtwcm9BdHRhY2hlZA==');
@$core.Deprecated('Use portDescriptor instead')
const Port$json = const {
  '1': 'Port',
  '2': const [
    const {'1': 'port', '3': 1, '4': 1, '5': 13, '10': 'port'},
  ],
};

/// Descriptor for `Port`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List portDescriptor = $convert.base64Decode('CgRQb3J0EhIKBHBvcnQYASABKA1SBHBvcnQ=');
