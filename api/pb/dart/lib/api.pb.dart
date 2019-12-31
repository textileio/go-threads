///
//  Generated code. Do not modify.
//  source: api.proto
//
// @dart = 2.3
// ignore_for_file: camel_case_types,non_constant_identifier_names,library_prefixes,unused_import,unused_shown_name,return_of_invalid_type

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

import 'api.pbenum.dart';

export 'api.pbenum.dart';

class NewStoreRequest extends $pb.GeneratedMessage {
  static final $pb.BuilderInfo _i = $pb.BuilderInfo('NewStoreRequest', package: const $pb.PackageName('api.pb'), createEmptyInstance: create)
    ..hasRequiredFields = false
  ;

  NewStoreRequest._() : super();
  factory NewStoreRequest() => create();
  factory NewStoreRequest.fromBuffer($core.List<$core.int> i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromBuffer(i, r);
  factory NewStoreRequest.fromJson($core.String i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromJson(i, r);
  NewStoreRequest clone() => NewStoreRequest()..mergeFromMessage(this);
  NewStoreRequest copyWith(void Function(NewStoreRequest) updates) => super.copyWith((message) => updates(message as NewStoreRequest));
  $pb.BuilderInfo get info_ => _i;
  @$core.pragma('dart2js:noInline')
  static NewStoreRequest create() => NewStoreRequest._();
  NewStoreRequest createEmptyInstance() => create();
  static $pb.PbList<NewStoreRequest> createRepeated() => $pb.PbList<NewStoreRequest>();
  @$core.pragma('dart2js:noInline')
  static NewStoreRequest getDefault() => _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<NewStoreRequest>(create);
  static NewStoreRequest _defaultInstance;
}

class NewStoreReply extends $pb.GeneratedMessage {
  static final $pb.BuilderInfo _i = $pb.BuilderInfo('NewStoreReply', package: const $pb.PackageName('api.pb'), createEmptyInstance: create)
    ..aOS(1, 'ID', protoName: 'ID')
    ..hasRequiredFields = false
  ;

  NewStoreReply._() : super();
  factory NewStoreReply() => create();
  factory NewStoreReply.fromBuffer($core.List<$core.int> i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromBuffer(i, r);
  factory NewStoreReply.fromJson($core.String i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromJson(i, r);
  NewStoreReply clone() => NewStoreReply()..mergeFromMessage(this);
  NewStoreReply copyWith(void Function(NewStoreReply) updates) => super.copyWith((message) => updates(message as NewStoreReply));
  $pb.BuilderInfo get info_ => _i;
  @$core.pragma('dart2js:noInline')
  static NewStoreReply create() => NewStoreReply._();
  NewStoreReply createEmptyInstance() => create();
  static $pb.PbList<NewStoreReply> createRepeated() => $pb.PbList<NewStoreReply>();
  @$core.pragma('dart2js:noInline')
  static NewStoreReply getDefault() => _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<NewStoreReply>(create);
  static NewStoreReply _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get iD => $_getSZ(0);
  @$pb.TagNumber(1)
  set iD($core.String v) { $_setString(0, v); }
  @$pb.TagNumber(1)
  $core.bool hasID() => $_has(0);
  @$pb.TagNumber(1)
  void clearID() => clearField(1);
}

class RegisterSchemaRequest extends $pb.GeneratedMessage {
  static final $pb.BuilderInfo _i = $pb.BuilderInfo('RegisterSchemaRequest', package: const $pb.PackageName('api.pb'), createEmptyInstance: create)
    ..aOS(1, 'storeID', protoName: 'storeID')
    ..aOS(2, 'name')
    ..aOS(3, 'schema')
    ..hasRequiredFields = false
  ;

  RegisterSchemaRequest._() : super();
  factory RegisterSchemaRequest() => create();
  factory RegisterSchemaRequest.fromBuffer($core.List<$core.int> i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromBuffer(i, r);
  factory RegisterSchemaRequest.fromJson($core.String i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromJson(i, r);
  RegisterSchemaRequest clone() => RegisterSchemaRequest()..mergeFromMessage(this);
  RegisterSchemaRequest copyWith(void Function(RegisterSchemaRequest) updates) => super.copyWith((message) => updates(message as RegisterSchemaRequest));
  $pb.BuilderInfo get info_ => _i;
  @$core.pragma('dart2js:noInline')
  static RegisterSchemaRequest create() => RegisterSchemaRequest._();
  RegisterSchemaRequest createEmptyInstance() => create();
  static $pb.PbList<RegisterSchemaRequest> createRepeated() => $pb.PbList<RegisterSchemaRequest>();
  @$core.pragma('dart2js:noInline')
  static RegisterSchemaRequest getDefault() => _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<RegisterSchemaRequest>(create);
  static RegisterSchemaRequest _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get storeID => $_getSZ(0);
  @$pb.TagNumber(1)
  set storeID($core.String v) { $_setString(0, v); }
  @$pb.TagNumber(1)
  $core.bool hasStoreID() => $_has(0);
  @$pb.TagNumber(1)
  void clearStoreID() => clearField(1);

  @$pb.TagNumber(2)
  $core.String get name => $_getSZ(1);
  @$pb.TagNumber(2)
  set name($core.String v) { $_setString(1, v); }
  @$pb.TagNumber(2)
  $core.bool hasName() => $_has(1);
  @$pb.TagNumber(2)
  void clearName() => clearField(2);

  @$pb.TagNumber(3)
  $core.String get schema => $_getSZ(2);
  @$pb.TagNumber(3)
  set schema($core.String v) { $_setString(2, v); }
  @$pb.TagNumber(3)
  $core.bool hasSchema() => $_has(2);
  @$pb.TagNumber(3)
  void clearSchema() => clearField(3);
}

class RegisterSchemaReply extends $pb.GeneratedMessage {
  static final $pb.BuilderInfo _i = $pb.BuilderInfo('RegisterSchemaReply', package: const $pb.PackageName('api.pb'), createEmptyInstance: create)
    ..hasRequiredFields = false
  ;

  RegisterSchemaReply._() : super();
  factory RegisterSchemaReply() => create();
  factory RegisterSchemaReply.fromBuffer($core.List<$core.int> i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromBuffer(i, r);
  factory RegisterSchemaReply.fromJson($core.String i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromJson(i, r);
  RegisterSchemaReply clone() => RegisterSchemaReply()..mergeFromMessage(this);
  RegisterSchemaReply copyWith(void Function(RegisterSchemaReply) updates) => super.copyWith((message) => updates(message as RegisterSchemaReply));
  $pb.BuilderInfo get info_ => _i;
  @$core.pragma('dart2js:noInline')
  static RegisterSchemaReply create() => RegisterSchemaReply._();
  RegisterSchemaReply createEmptyInstance() => create();
  static $pb.PbList<RegisterSchemaReply> createRepeated() => $pb.PbList<RegisterSchemaReply>();
  @$core.pragma('dart2js:noInline')
  static RegisterSchemaReply getDefault() => _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<RegisterSchemaReply>(create);
  static RegisterSchemaReply _defaultInstance;
}

class StartRequest extends $pb.GeneratedMessage {
  static final $pb.BuilderInfo _i = $pb.BuilderInfo('StartRequest', package: const $pb.PackageName('api.pb'), createEmptyInstance: create)
    ..aOS(1, 'storeID', protoName: 'storeID')
    ..hasRequiredFields = false
  ;

  StartRequest._() : super();
  factory StartRequest() => create();
  factory StartRequest.fromBuffer($core.List<$core.int> i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromBuffer(i, r);
  factory StartRequest.fromJson($core.String i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromJson(i, r);
  StartRequest clone() => StartRequest()..mergeFromMessage(this);
  StartRequest copyWith(void Function(StartRequest) updates) => super.copyWith((message) => updates(message as StartRequest));
  $pb.BuilderInfo get info_ => _i;
  @$core.pragma('dart2js:noInline')
  static StartRequest create() => StartRequest._();
  StartRequest createEmptyInstance() => create();
  static $pb.PbList<StartRequest> createRepeated() => $pb.PbList<StartRequest>();
  @$core.pragma('dart2js:noInline')
  static StartRequest getDefault() => _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<StartRequest>(create);
  static StartRequest _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get storeID => $_getSZ(0);
  @$pb.TagNumber(1)
  set storeID($core.String v) { $_setString(0, v); }
  @$pb.TagNumber(1)
  $core.bool hasStoreID() => $_has(0);
  @$pb.TagNumber(1)
  void clearStoreID() => clearField(1);
}

class StartReply extends $pb.GeneratedMessage {
  static final $pb.BuilderInfo _i = $pb.BuilderInfo('StartReply', package: const $pb.PackageName('api.pb'), createEmptyInstance: create)
    ..hasRequiredFields = false
  ;

  StartReply._() : super();
  factory StartReply() => create();
  factory StartReply.fromBuffer($core.List<$core.int> i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromBuffer(i, r);
  factory StartReply.fromJson($core.String i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromJson(i, r);
  StartReply clone() => StartReply()..mergeFromMessage(this);
  StartReply copyWith(void Function(StartReply) updates) => super.copyWith((message) => updates(message as StartReply));
  $pb.BuilderInfo get info_ => _i;
  @$core.pragma('dart2js:noInline')
  static StartReply create() => StartReply._();
  StartReply createEmptyInstance() => create();
  static $pb.PbList<StartReply> createRepeated() => $pb.PbList<StartReply>();
  @$core.pragma('dart2js:noInline')
  static StartReply getDefault() => _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<StartReply>(create);
  static StartReply _defaultInstance;
}

class StartFromAddressRequest extends $pb.GeneratedMessage {
  static final $pb.BuilderInfo _i = $pb.BuilderInfo('StartFromAddressRequest', package: const $pb.PackageName('api.pb'), createEmptyInstance: create)
    ..aOS(1, 'storeID', protoName: 'storeID')
    ..aOS(2, 'address')
    ..a<$core.List<$core.int>>(3, 'followKey', $pb.PbFieldType.OY, protoName: 'followKey')
    ..a<$core.List<$core.int>>(4, 'readKey', $pb.PbFieldType.OY, protoName: 'readKey')
    ..hasRequiredFields = false
  ;

  StartFromAddressRequest._() : super();
  factory StartFromAddressRequest() => create();
  factory StartFromAddressRequest.fromBuffer($core.List<$core.int> i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromBuffer(i, r);
  factory StartFromAddressRequest.fromJson($core.String i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromJson(i, r);
  StartFromAddressRequest clone() => StartFromAddressRequest()..mergeFromMessage(this);
  StartFromAddressRequest copyWith(void Function(StartFromAddressRequest) updates) => super.copyWith((message) => updates(message as StartFromAddressRequest));
  $pb.BuilderInfo get info_ => _i;
  @$core.pragma('dart2js:noInline')
  static StartFromAddressRequest create() => StartFromAddressRequest._();
  StartFromAddressRequest createEmptyInstance() => create();
  static $pb.PbList<StartFromAddressRequest> createRepeated() => $pb.PbList<StartFromAddressRequest>();
  @$core.pragma('dart2js:noInline')
  static StartFromAddressRequest getDefault() => _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<StartFromAddressRequest>(create);
  static StartFromAddressRequest _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get storeID => $_getSZ(0);
  @$pb.TagNumber(1)
  set storeID($core.String v) { $_setString(0, v); }
  @$pb.TagNumber(1)
  $core.bool hasStoreID() => $_has(0);
  @$pb.TagNumber(1)
  void clearStoreID() => clearField(1);

  @$pb.TagNumber(2)
  $core.String get address => $_getSZ(1);
  @$pb.TagNumber(2)
  set address($core.String v) { $_setString(1, v); }
  @$pb.TagNumber(2)
  $core.bool hasAddress() => $_has(1);
  @$pb.TagNumber(2)
  void clearAddress() => clearField(2);

  @$pb.TagNumber(3)
  $core.List<$core.int> get followKey => $_getN(2);
  @$pb.TagNumber(3)
  set followKey($core.List<$core.int> v) { $_setBytes(2, v); }
  @$pb.TagNumber(3)
  $core.bool hasFollowKey() => $_has(2);
  @$pb.TagNumber(3)
  void clearFollowKey() => clearField(3);

  @$pb.TagNumber(4)
  $core.List<$core.int> get readKey => $_getN(3);
  @$pb.TagNumber(4)
  set readKey($core.List<$core.int> v) { $_setBytes(3, v); }
  @$pb.TagNumber(4)
  $core.bool hasReadKey() => $_has(3);
  @$pb.TagNumber(4)
  void clearReadKey() => clearField(4);
}

class StartFromAddressReply extends $pb.GeneratedMessage {
  static final $pb.BuilderInfo _i = $pb.BuilderInfo('StartFromAddressReply', package: const $pb.PackageName('api.pb'), createEmptyInstance: create)
    ..hasRequiredFields = false
  ;

  StartFromAddressReply._() : super();
  factory StartFromAddressReply() => create();
  factory StartFromAddressReply.fromBuffer($core.List<$core.int> i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromBuffer(i, r);
  factory StartFromAddressReply.fromJson($core.String i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromJson(i, r);
  StartFromAddressReply clone() => StartFromAddressReply()..mergeFromMessage(this);
  StartFromAddressReply copyWith(void Function(StartFromAddressReply) updates) => super.copyWith((message) => updates(message as StartFromAddressReply));
  $pb.BuilderInfo get info_ => _i;
  @$core.pragma('dart2js:noInline')
  static StartFromAddressReply create() => StartFromAddressReply._();
  StartFromAddressReply createEmptyInstance() => create();
  static $pb.PbList<StartFromAddressReply> createRepeated() => $pb.PbList<StartFromAddressReply>();
  @$core.pragma('dart2js:noInline')
  static StartFromAddressReply getDefault() => _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<StartFromAddressReply>(create);
  static StartFromAddressReply _defaultInstance;
}

class GetStoreLinkRequest extends $pb.GeneratedMessage {
  static final $pb.BuilderInfo _i = $pb.BuilderInfo('GetStoreLinkRequest', package: const $pb.PackageName('api.pb'), createEmptyInstance: create)
    ..aOS(1, 'storeID', protoName: 'storeID')
    ..hasRequiredFields = false
  ;

  GetStoreLinkRequest._() : super();
  factory GetStoreLinkRequest() => create();
  factory GetStoreLinkRequest.fromBuffer($core.List<$core.int> i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromBuffer(i, r);
  factory GetStoreLinkRequest.fromJson($core.String i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromJson(i, r);
  GetStoreLinkRequest clone() => GetStoreLinkRequest()..mergeFromMessage(this);
  GetStoreLinkRequest copyWith(void Function(GetStoreLinkRequest) updates) => super.copyWith((message) => updates(message as GetStoreLinkRequest));
  $pb.BuilderInfo get info_ => _i;
  @$core.pragma('dart2js:noInline')
  static GetStoreLinkRequest create() => GetStoreLinkRequest._();
  GetStoreLinkRequest createEmptyInstance() => create();
  static $pb.PbList<GetStoreLinkRequest> createRepeated() => $pb.PbList<GetStoreLinkRequest>();
  @$core.pragma('dart2js:noInline')
  static GetStoreLinkRequest getDefault() => _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<GetStoreLinkRequest>(create);
  static GetStoreLinkRequest _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get storeID => $_getSZ(0);
  @$pb.TagNumber(1)
  set storeID($core.String v) { $_setString(0, v); }
  @$pb.TagNumber(1)
  $core.bool hasStoreID() => $_has(0);
  @$pb.TagNumber(1)
  void clearStoreID() => clearField(1);
}

class GetStoreLinkReply extends $pb.GeneratedMessage {
  static final $pb.BuilderInfo _i = $pb.BuilderInfo('GetStoreLinkReply', package: const $pb.PackageName('api.pb'), createEmptyInstance: create)
    ..pPS(1, 'addresses')
    ..a<$core.List<$core.int>>(2, 'followKey', $pb.PbFieldType.OY, protoName: 'followKey')
    ..a<$core.List<$core.int>>(3, 'readKey', $pb.PbFieldType.OY, protoName: 'readKey')
    ..hasRequiredFields = false
  ;

  GetStoreLinkReply._() : super();
  factory GetStoreLinkReply() => create();
  factory GetStoreLinkReply.fromBuffer($core.List<$core.int> i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromBuffer(i, r);
  factory GetStoreLinkReply.fromJson($core.String i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromJson(i, r);
  GetStoreLinkReply clone() => GetStoreLinkReply()..mergeFromMessage(this);
  GetStoreLinkReply copyWith(void Function(GetStoreLinkReply) updates) => super.copyWith((message) => updates(message as GetStoreLinkReply));
  $pb.BuilderInfo get info_ => _i;
  @$core.pragma('dart2js:noInline')
  static GetStoreLinkReply create() => GetStoreLinkReply._();
  GetStoreLinkReply createEmptyInstance() => create();
  static $pb.PbList<GetStoreLinkReply> createRepeated() => $pb.PbList<GetStoreLinkReply>();
  @$core.pragma('dart2js:noInline')
  static GetStoreLinkReply getDefault() => _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<GetStoreLinkReply>(create);
  static GetStoreLinkReply _defaultInstance;

  @$pb.TagNumber(1)
  $core.List<$core.String> get addresses => $_getList(0);

  @$pb.TagNumber(2)
  $core.List<$core.int> get followKey => $_getN(1);
  @$pb.TagNumber(2)
  set followKey($core.List<$core.int> v) { $_setBytes(1, v); }
  @$pb.TagNumber(2)
  $core.bool hasFollowKey() => $_has(1);
  @$pb.TagNumber(2)
  void clearFollowKey() => clearField(2);

  @$pb.TagNumber(3)
  $core.List<$core.int> get readKey => $_getN(2);
  @$pb.TagNumber(3)
  set readKey($core.List<$core.int> v) { $_setBytes(2, v); }
  @$pb.TagNumber(3)
  $core.bool hasReadKey() => $_has(2);
  @$pb.TagNumber(3)
  void clearReadKey() => clearField(3);
}

class ModelCreateRequest extends $pb.GeneratedMessage {
  static final $pb.BuilderInfo _i = $pb.BuilderInfo('ModelCreateRequest', package: const $pb.PackageName('api.pb'), createEmptyInstance: create)
    ..aOS(1, 'storeID', protoName: 'storeID')
    ..aOS(2, 'modelName', protoName: 'modelName')
    ..pPS(3, 'values')
    ..hasRequiredFields = false
  ;

  ModelCreateRequest._() : super();
  factory ModelCreateRequest() => create();
  factory ModelCreateRequest.fromBuffer($core.List<$core.int> i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromBuffer(i, r);
  factory ModelCreateRequest.fromJson($core.String i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromJson(i, r);
  ModelCreateRequest clone() => ModelCreateRequest()..mergeFromMessage(this);
  ModelCreateRequest copyWith(void Function(ModelCreateRequest) updates) => super.copyWith((message) => updates(message as ModelCreateRequest));
  $pb.BuilderInfo get info_ => _i;
  @$core.pragma('dart2js:noInline')
  static ModelCreateRequest create() => ModelCreateRequest._();
  ModelCreateRequest createEmptyInstance() => create();
  static $pb.PbList<ModelCreateRequest> createRepeated() => $pb.PbList<ModelCreateRequest>();
  @$core.pragma('dart2js:noInline')
  static ModelCreateRequest getDefault() => _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ModelCreateRequest>(create);
  static ModelCreateRequest _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get storeID => $_getSZ(0);
  @$pb.TagNumber(1)
  set storeID($core.String v) { $_setString(0, v); }
  @$pb.TagNumber(1)
  $core.bool hasStoreID() => $_has(0);
  @$pb.TagNumber(1)
  void clearStoreID() => clearField(1);

  @$pb.TagNumber(2)
  $core.String get modelName => $_getSZ(1);
  @$pb.TagNumber(2)
  set modelName($core.String v) { $_setString(1, v); }
  @$pb.TagNumber(2)
  $core.bool hasModelName() => $_has(1);
  @$pb.TagNumber(2)
  void clearModelName() => clearField(2);

  @$pb.TagNumber(3)
  $core.List<$core.String> get values => $_getList(2);
}

class ModelCreateReply extends $pb.GeneratedMessage {
  static final $pb.BuilderInfo _i = $pb.BuilderInfo('ModelCreateReply', package: const $pb.PackageName('api.pb'), createEmptyInstance: create)
    ..pPS(1, 'entities')
    ..hasRequiredFields = false
  ;

  ModelCreateReply._() : super();
  factory ModelCreateReply() => create();
  factory ModelCreateReply.fromBuffer($core.List<$core.int> i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromBuffer(i, r);
  factory ModelCreateReply.fromJson($core.String i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromJson(i, r);
  ModelCreateReply clone() => ModelCreateReply()..mergeFromMessage(this);
  ModelCreateReply copyWith(void Function(ModelCreateReply) updates) => super.copyWith((message) => updates(message as ModelCreateReply));
  $pb.BuilderInfo get info_ => _i;
  @$core.pragma('dart2js:noInline')
  static ModelCreateReply create() => ModelCreateReply._();
  ModelCreateReply createEmptyInstance() => create();
  static $pb.PbList<ModelCreateReply> createRepeated() => $pb.PbList<ModelCreateReply>();
  @$core.pragma('dart2js:noInline')
  static ModelCreateReply getDefault() => _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ModelCreateReply>(create);
  static ModelCreateReply _defaultInstance;

  @$pb.TagNumber(1)
  $core.List<$core.String> get entities => $_getList(0);
}

class ModelSaveRequest extends $pb.GeneratedMessage {
  static final $pb.BuilderInfo _i = $pb.BuilderInfo('ModelSaveRequest', package: const $pb.PackageName('api.pb'), createEmptyInstance: create)
    ..aOS(1, 'storeID', protoName: 'storeID')
    ..aOS(2, 'modelName', protoName: 'modelName')
    ..pPS(3, 'values')
    ..hasRequiredFields = false
  ;

  ModelSaveRequest._() : super();
  factory ModelSaveRequest() => create();
  factory ModelSaveRequest.fromBuffer($core.List<$core.int> i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromBuffer(i, r);
  factory ModelSaveRequest.fromJson($core.String i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromJson(i, r);
  ModelSaveRequest clone() => ModelSaveRequest()..mergeFromMessage(this);
  ModelSaveRequest copyWith(void Function(ModelSaveRequest) updates) => super.copyWith((message) => updates(message as ModelSaveRequest));
  $pb.BuilderInfo get info_ => _i;
  @$core.pragma('dart2js:noInline')
  static ModelSaveRequest create() => ModelSaveRequest._();
  ModelSaveRequest createEmptyInstance() => create();
  static $pb.PbList<ModelSaveRequest> createRepeated() => $pb.PbList<ModelSaveRequest>();
  @$core.pragma('dart2js:noInline')
  static ModelSaveRequest getDefault() => _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ModelSaveRequest>(create);
  static ModelSaveRequest _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get storeID => $_getSZ(0);
  @$pb.TagNumber(1)
  set storeID($core.String v) { $_setString(0, v); }
  @$pb.TagNumber(1)
  $core.bool hasStoreID() => $_has(0);
  @$pb.TagNumber(1)
  void clearStoreID() => clearField(1);

  @$pb.TagNumber(2)
  $core.String get modelName => $_getSZ(1);
  @$pb.TagNumber(2)
  set modelName($core.String v) { $_setString(1, v); }
  @$pb.TagNumber(2)
  $core.bool hasModelName() => $_has(1);
  @$pb.TagNumber(2)
  void clearModelName() => clearField(2);

  @$pb.TagNumber(3)
  $core.List<$core.String> get values => $_getList(2);
}

class ModelSaveReply extends $pb.GeneratedMessage {
  static final $pb.BuilderInfo _i = $pb.BuilderInfo('ModelSaveReply', package: const $pb.PackageName('api.pb'), createEmptyInstance: create)
    ..hasRequiredFields = false
  ;

  ModelSaveReply._() : super();
  factory ModelSaveReply() => create();
  factory ModelSaveReply.fromBuffer($core.List<$core.int> i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromBuffer(i, r);
  factory ModelSaveReply.fromJson($core.String i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromJson(i, r);
  ModelSaveReply clone() => ModelSaveReply()..mergeFromMessage(this);
  ModelSaveReply copyWith(void Function(ModelSaveReply) updates) => super.copyWith((message) => updates(message as ModelSaveReply));
  $pb.BuilderInfo get info_ => _i;
  @$core.pragma('dart2js:noInline')
  static ModelSaveReply create() => ModelSaveReply._();
  ModelSaveReply createEmptyInstance() => create();
  static $pb.PbList<ModelSaveReply> createRepeated() => $pb.PbList<ModelSaveReply>();
  @$core.pragma('dart2js:noInline')
  static ModelSaveReply getDefault() => _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ModelSaveReply>(create);
  static ModelSaveReply _defaultInstance;
}

class ModelDeleteRequest extends $pb.GeneratedMessage {
  static final $pb.BuilderInfo _i = $pb.BuilderInfo('ModelDeleteRequest', package: const $pb.PackageName('api.pb'), createEmptyInstance: create)
    ..aOS(1, 'storeID', protoName: 'storeID')
    ..aOS(2, 'modelName', protoName: 'modelName')
    ..pPS(3, 'entityIDs', protoName: 'entityIDs')
    ..hasRequiredFields = false
  ;

  ModelDeleteRequest._() : super();
  factory ModelDeleteRequest() => create();
  factory ModelDeleteRequest.fromBuffer($core.List<$core.int> i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromBuffer(i, r);
  factory ModelDeleteRequest.fromJson($core.String i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromJson(i, r);
  ModelDeleteRequest clone() => ModelDeleteRequest()..mergeFromMessage(this);
  ModelDeleteRequest copyWith(void Function(ModelDeleteRequest) updates) => super.copyWith((message) => updates(message as ModelDeleteRequest));
  $pb.BuilderInfo get info_ => _i;
  @$core.pragma('dart2js:noInline')
  static ModelDeleteRequest create() => ModelDeleteRequest._();
  ModelDeleteRequest createEmptyInstance() => create();
  static $pb.PbList<ModelDeleteRequest> createRepeated() => $pb.PbList<ModelDeleteRequest>();
  @$core.pragma('dart2js:noInline')
  static ModelDeleteRequest getDefault() => _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ModelDeleteRequest>(create);
  static ModelDeleteRequest _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get storeID => $_getSZ(0);
  @$pb.TagNumber(1)
  set storeID($core.String v) { $_setString(0, v); }
  @$pb.TagNumber(1)
  $core.bool hasStoreID() => $_has(0);
  @$pb.TagNumber(1)
  void clearStoreID() => clearField(1);

  @$pb.TagNumber(2)
  $core.String get modelName => $_getSZ(1);
  @$pb.TagNumber(2)
  set modelName($core.String v) { $_setString(1, v); }
  @$pb.TagNumber(2)
  $core.bool hasModelName() => $_has(1);
  @$pb.TagNumber(2)
  void clearModelName() => clearField(2);

  @$pb.TagNumber(3)
  $core.List<$core.String> get entityIDs => $_getList(2);
}

class ModelDeleteReply extends $pb.GeneratedMessage {
  static final $pb.BuilderInfo _i = $pb.BuilderInfo('ModelDeleteReply', package: const $pb.PackageName('api.pb'), createEmptyInstance: create)
    ..hasRequiredFields = false
  ;

  ModelDeleteReply._() : super();
  factory ModelDeleteReply() => create();
  factory ModelDeleteReply.fromBuffer($core.List<$core.int> i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromBuffer(i, r);
  factory ModelDeleteReply.fromJson($core.String i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromJson(i, r);
  ModelDeleteReply clone() => ModelDeleteReply()..mergeFromMessage(this);
  ModelDeleteReply copyWith(void Function(ModelDeleteReply) updates) => super.copyWith((message) => updates(message as ModelDeleteReply));
  $pb.BuilderInfo get info_ => _i;
  @$core.pragma('dart2js:noInline')
  static ModelDeleteReply create() => ModelDeleteReply._();
  ModelDeleteReply createEmptyInstance() => create();
  static $pb.PbList<ModelDeleteReply> createRepeated() => $pb.PbList<ModelDeleteReply>();
  @$core.pragma('dart2js:noInline')
  static ModelDeleteReply getDefault() => _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ModelDeleteReply>(create);
  static ModelDeleteReply _defaultInstance;
}

class ModelHasRequest extends $pb.GeneratedMessage {
  static final $pb.BuilderInfo _i = $pb.BuilderInfo('ModelHasRequest', package: const $pb.PackageName('api.pb'), createEmptyInstance: create)
    ..aOS(1, 'storeID', protoName: 'storeID')
    ..aOS(2, 'modelName', protoName: 'modelName')
    ..pPS(3, 'entityIDs', protoName: 'entityIDs')
    ..hasRequiredFields = false
  ;

  ModelHasRequest._() : super();
  factory ModelHasRequest() => create();
  factory ModelHasRequest.fromBuffer($core.List<$core.int> i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromBuffer(i, r);
  factory ModelHasRequest.fromJson($core.String i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromJson(i, r);
  ModelHasRequest clone() => ModelHasRequest()..mergeFromMessage(this);
  ModelHasRequest copyWith(void Function(ModelHasRequest) updates) => super.copyWith((message) => updates(message as ModelHasRequest));
  $pb.BuilderInfo get info_ => _i;
  @$core.pragma('dart2js:noInline')
  static ModelHasRequest create() => ModelHasRequest._();
  ModelHasRequest createEmptyInstance() => create();
  static $pb.PbList<ModelHasRequest> createRepeated() => $pb.PbList<ModelHasRequest>();
  @$core.pragma('dart2js:noInline')
  static ModelHasRequest getDefault() => _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ModelHasRequest>(create);
  static ModelHasRequest _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get storeID => $_getSZ(0);
  @$pb.TagNumber(1)
  set storeID($core.String v) { $_setString(0, v); }
  @$pb.TagNumber(1)
  $core.bool hasStoreID() => $_has(0);
  @$pb.TagNumber(1)
  void clearStoreID() => clearField(1);

  @$pb.TagNumber(2)
  $core.String get modelName => $_getSZ(1);
  @$pb.TagNumber(2)
  set modelName($core.String v) { $_setString(1, v); }
  @$pb.TagNumber(2)
  $core.bool hasModelName() => $_has(1);
  @$pb.TagNumber(2)
  void clearModelName() => clearField(2);

  @$pb.TagNumber(3)
  $core.List<$core.String> get entityIDs => $_getList(2);
}

class ModelHasReply extends $pb.GeneratedMessage {
  static final $pb.BuilderInfo _i = $pb.BuilderInfo('ModelHasReply', package: const $pb.PackageName('api.pb'), createEmptyInstance: create)
    ..aOB(1, 'exists')
    ..hasRequiredFields = false
  ;

  ModelHasReply._() : super();
  factory ModelHasReply() => create();
  factory ModelHasReply.fromBuffer($core.List<$core.int> i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromBuffer(i, r);
  factory ModelHasReply.fromJson($core.String i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromJson(i, r);
  ModelHasReply clone() => ModelHasReply()..mergeFromMessage(this);
  ModelHasReply copyWith(void Function(ModelHasReply) updates) => super.copyWith((message) => updates(message as ModelHasReply));
  $pb.BuilderInfo get info_ => _i;
  @$core.pragma('dart2js:noInline')
  static ModelHasReply create() => ModelHasReply._();
  ModelHasReply createEmptyInstance() => create();
  static $pb.PbList<ModelHasReply> createRepeated() => $pb.PbList<ModelHasReply>();
  @$core.pragma('dart2js:noInline')
  static ModelHasReply getDefault() => _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ModelHasReply>(create);
  static ModelHasReply _defaultInstance;

  @$pb.TagNumber(1)
  $core.bool get exists => $_getBF(0);
  @$pb.TagNumber(1)
  set exists($core.bool v) { $_setBool(0, v); }
  @$pb.TagNumber(1)
  $core.bool hasExists() => $_has(0);
  @$pb.TagNumber(1)
  void clearExists() => clearField(1);
}

class ModelFindRequest extends $pb.GeneratedMessage {
  static final $pb.BuilderInfo _i = $pb.BuilderInfo('ModelFindRequest', package: const $pb.PackageName('api.pb'), createEmptyInstance: create)
    ..aOS(1, 'storeID', protoName: 'storeID')
    ..aOS(2, 'modelName', protoName: 'modelName')
    ..a<$core.List<$core.int>>(3, 'queryJSON', $pb.PbFieldType.OY, protoName: 'queryJSON')
    ..hasRequiredFields = false
  ;

  ModelFindRequest._() : super();
  factory ModelFindRequest() => create();
  factory ModelFindRequest.fromBuffer($core.List<$core.int> i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromBuffer(i, r);
  factory ModelFindRequest.fromJson($core.String i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromJson(i, r);
  ModelFindRequest clone() => ModelFindRequest()..mergeFromMessage(this);
  ModelFindRequest copyWith(void Function(ModelFindRequest) updates) => super.copyWith((message) => updates(message as ModelFindRequest));
  $pb.BuilderInfo get info_ => _i;
  @$core.pragma('dart2js:noInline')
  static ModelFindRequest create() => ModelFindRequest._();
  ModelFindRequest createEmptyInstance() => create();
  static $pb.PbList<ModelFindRequest> createRepeated() => $pb.PbList<ModelFindRequest>();
  @$core.pragma('dart2js:noInline')
  static ModelFindRequest getDefault() => _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ModelFindRequest>(create);
  static ModelFindRequest _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get storeID => $_getSZ(0);
  @$pb.TagNumber(1)
  set storeID($core.String v) { $_setString(0, v); }
  @$pb.TagNumber(1)
  $core.bool hasStoreID() => $_has(0);
  @$pb.TagNumber(1)
  void clearStoreID() => clearField(1);

  @$pb.TagNumber(2)
  $core.String get modelName => $_getSZ(1);
  @$pb.TagNumber(2)
  set modelName($core.String v) { $_setString(1, v); }
  @$pb.TagNumber(2)
  $core.bool hasModelName() => $_has(1);
  @$pb.TagNumber(2)
  void clearModelName() => clearField(2);

  @$pb.TagNumber(3)
  $core.List<$core.int> get queryJSON => $_getN(2);
  @$pb.TagNumber(3)
  set queryJSON($core.List<$core.int> v) { $_setBytes(2, v); }
  @$pb.TagNumber(3)
  $core.bool hasQueryJSON() => $_has(2);
  @$pb.TagNumber(3)
  void clearQueryJSON() => clearField(3);
}

class ModelFindReply extends $pb.GeneratedMessage {
  static final $pb.BuilderInfo _i = $pb.BuilderInfo('ModelFindReply', package: const $pb.PackageName('api.pb'), createEmptyInstance: create)
    ..p<$core.List<$core.int>>(1, 'entities', $pb.PbFieldType.PY)
    ..hasRequiredFields = false
  ;

  ModelFindReply._() : super();
  factory ModelFindReply() => create();
  factory ModelFindReply.fromBuffer($core.List<$core.int> i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromBuffer(i, r);
  factory ModelFindReply.fromJson($core.String i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromJson(i, r);
  ModelFindReply clone() => ModelFindReply()..mergeFromMessage(this);
  ModelFindReply copyWith(void Function(ModelFindReply) updates) => super.copyWith((message) => updates(message as ModelFindReply));
  $pb.BuilderInfo get info_ => _i;
  @$core.pragma('dart2js:noInline')
  static ModelFindReply create() => ModelFindReply._();
  ModelFindReply createEmptyInstance() => create();
  static $pb.PbList<ModelFindReply> createRepeated() => $pb.PbList<ModelFindReply>();
  @$core.pragma('dart2js:noInline')
  static ModelFindReply getDefault() => _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ModelFindReply>(create);
  static ModelFindReply _defaultInstance;

  @$pb.TagNumber(1)
  $core.List<$core.List<$core.int>> get entities => $_getList(0);
}

class ModelFindByIDRequest extends $pb.GeneratedMessage {
  static final $pb.BuilderInfo _i = $pb.BuilderInfo('ModelFindByIDRequest', package: const $pb.PackageName('api.pb'), createEmptyInstance: create)
    ..aOS(1, 'storeID', protoName: 'storeID')
    ..aOS(2, 'modelName', protoName: 'modelName')
    ..aOS(3, 'entityID', protoName: 'entityID')
    ..hasRequiredFields = false
  ;

  ModelFindByIDRequest._() : super();
  factory ModelFindByIDRequest() => create();
  factory ModelFindByIDRequest.fromBuffer($core.List<$core.int> i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromBuffer(i, r);
  factory ModelFindByIDRequest.fromJson($core.String i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromJson(i, r);
  ModelFindByIDRequest clone() => ModelFindByIDRequest()..mergeFromMessage(this);
  ModelFindByIDRequest copyWith(void Function(ModelFindByIDRequest) updates) => super.copyWith((message) => updates(message as ModelFindByIDRequest));
  $pb.BuilderInfo get info_ => _i;
  @$core.pragma('dart2js:noInline')
  static ModelFindByIDRequest create() => ModelFindByIDRequest._();
  ModelFindByIDRequest createEmptyInstance() => create();
  static $pb.PbList<ModelFindByIDRequest> createRepeated() => $pb.PbList<ModelFindByIDRequest>();
  @$core.pragma('dart2js:noInline')
  static ModelFindByIDRequest getDefault() => _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ModelFindByIDRequest>(create);
  static ModelFindByIDRequest _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get storeID => $_getSZ(0);
  @$pb.TagNumber(1)
  set storeID($core.String v) { $_setString(0, v); }
  @$pb.TagNumber(1)
  $core.bool hasStoreID() => $_has(0);
  @$pb.TagNumber(1)
  void clearStoreID() => clearField(1);

  @$pb.TagNumber(2)
  $core.String get modelName => $_getSZ(1);
  @$pb.TagNumber(2)
  set modelName($core.String v) { $_setString(1, v); }
  @$pb.TagNumber(2)
  $core.bool hasModelName() => $_has(1);
  @$pb.TagNumber(2)
  void clearModelName() => clearField(2);

  @$pb.TagNumber(3)
  $core.String get entityID => $_getSZ(2);
  @$pb.TagNumber(3)
  set entityID($core.String v) { $_setString(2, v); }
  @$pb.TagNumber(3)
  $core.bool hasEntityID() => $_has(2);
  @$pb.TagNumber(3)
  void clearEntityID() => clearField(3);
}

class ModelFindByIDReply extends $pb.GeneratedMessage {
  static final $pb.BuilderInfo _i = $pb.BuilderInfo('ModelFindByIDReply', package: const $pb.PackageName('api.pb'), createEmptyInstance: create)
    ..aOS(1, 'entity')
    ..hasRequiredFields = false
  ;

  ModelFindByIDReply._() : super();
  factory ModelFindByIDReply() => create();
  factory ModelFindByIDReply.fromBuffer($core.List<$core.int> i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromBuffer(i, r);
  factory ModelFindByIDReply.fromJson($core.String i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromJson(i, r);
  ModelFindByIDReply clone() => ModelFindByIDReply()..mergeFromMessage(this);
  ModelFindByIDReply copyWith(void Function(ModelFindByIDReply) updates) => super.copyWith((message) => updates(message as ModelFindByIDReply));
  $pb.BuilderInfo get info_ => _i;
  @$core.pragma('dart2js:noInline')
  static ModelFindByIDReply create() => ModelFindByIDReply._();
  ModelFindByIDReply createEmptyInstance() => create();
  static $pb.PbList<ModelFindByIDReply> createRepeated() => $pb.PbList<ModelFindByIDReply>();
  @$core.pragma('dart2js:noInline')
  static ModelFindByIDReply getDefault() => _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ModelFindByIDReply>(create);
  static ModelFindByIDReply _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get entity => $_getSZ(0);
  @$pb.TagNumber(1)
  set entity($core.String v) { $_setString(0, v); }
  @$pb.TagNumber(1)
  $core.bool hasEntity() => $_has(0);
  @$pb.TagNumber(1)
  void clearEntity() => clearField(1);
}

class StartTransactionRequest extends $pb.GeneratedMessage {
  static final $pb.BuilderInfo _i = $pb.BuilderInfo('StartTransactionRequest', package: const $pb.PackageName('api.pb'), createEmptyInstance: create)
    ..aOS(1, 'storeID', protoName: 'storeID')
    ..aOS(2, 'modelName', protoName: 'modelName')
    ..hasRequiredFields = false
  ;

  StartTransactionRequest._() : super();
  factory StartTransactionRequest() => create();
  factory StartTransactionRequest.fromBuffer($core.List<$core.int> i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromBuffer(i, r);
  factory StartTransactionRequest.fromJson($core.String i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromJson(i, r);
  StartTransactionRequest clone() => StartTransactionRequest()..mergeFromMessage(this);
  StartTransactionRequest copyWith(void Function(StartTransactionRequest) updates) => super.copyWith((message) => updates(message as StartTransactionRequest));
  $pb.BuilderInfo get info_ => _i;
  @$core.pragma('dart2js:noInline')
  static StartTransactionRequest create() => StartTransactionRequest._();
  StartTransactionRequest createEmptyInstance() => create();
  static $pb.PbList<StartTransactionRequest> createRepeated() => $pb.PbList<StartTransactionRequest>();
  @$core.pragma('dart2js:noInline')
  static StartTransactionRequest getDefault() => _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<StartTransactionRequest>(create);
  static StartTransactionRequest _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get storeID => $_getSZ(0);
  @$pb.TagNumber(1)
  set storeID($core.String v) { $_setString(0, v); }
  @$pb.TagNumber(1)
  $core.bool hasStoreID() => $_has(0);
  @$pb.TagNumber(1)
  void clearStoreID() => clearField(1);

  @$pb.TagNumber(2)
  $core.String get modelName => $_getSZ(1);
  @$pb.TagNumber(2)
  set modelName($core.String v) { $_setString(1, v); }
  @$pb.TagNumber(2)
  $core.bool hasModelName() => $_has(1);
  @$pb.TagNumber(2)
  void clearModelName() => clearField(2);
}

enum ReadTransactionRequest_Option {
  startTransactionRequest, 
  modelHasRequest, 
  modelFindRequest, 
  modelFindByIDRequest, 
  notSet
}

class ReadTransactionRequest extends $pb.GeneratedMessage {
  static const $core.Map<$core.int, ReadTransactionRequest_Option> _ReadTransactionRequest_OptionByTag = {
    1 : ReadTransactionRequest_Option.startTransactionRequest,
    2 : ReadTransactionRequest_Option.modelHasRequest,
    3 : ReadTransactionRequest_Option.modelFindRequest,
    4 : ReadTransactionRequest_Option.modelFindByIDRequest,
    0 : ReadTransactionRequest_Option.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo('ReadTransactionRequest', package: const $pb.PackageName('api.pb'), createEmptyInstance: create)
    ..oo(0, [1, 2, 3, 4])
    ..aOM<StartTransactionRequest>(1, 'startTransactionRequest', protoName: 'startTransactionRequest', subBuilder: StartTransactionRequest.create)
    ..aOM<ModelHasRequest>(2, 'modelHasRequest', protoName: 'modelHasRequest', subBuilder: ModelHasRequest.create)
    ..aOM<ModelFindRequest>(3, 'modelFindRequest', protoName: 'modelFindRequest', subBuilder: ModelFindRequest.create)
    ..aOM<ModelFindByIDRequest>(4, 'modelFindByIDRequest', protoName: 'modelFindByIDRequest', subBuilder: ModelFindByIDRequest.create)
    ..hasRequiredFields = false
  ;

  ReadTransactionRequest._() : super();
  factory ReadTransactionRequest() => create();
  factory ReadTransactionRequest.fromBuffer($core.List<$core.int> i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromBuffer(i, r);
  factory ReadTransactionRequest.fromJson($core.String i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromJson(i, r);
  ReadTransactionRequest clone() => ReadTransactionRequest()..mergeFromMessage(this);
  ReadTransactionRequest copyWith(void Function(ReadTransactionRequest) updates) => super.copyWith((message) => updates(message as ReadTransactionRequest));
  $pb.BuilderInfo get info_ => _i;
  @$core.pragma('dart2js:noInline')
  static ReadTransactionRequest create() => ReadTransactionRequest._();
  ReadTransactionRequest createEmptyInstance() => create();
  static $pb.PbList<ReadTransactionRequest> createRepeated() => $pb.PbList<ReadTransactionRequest>();
  @$core.pragma('dart2js:noInline')
  static ReadTransactionRequest getDefault() => _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ReadTransactionRequest>(create);
  static ReadTransactionRequest _defaultInstance;

  ReadTransactionRequest_Option whichOption() => _ReadTransactionRequest_OptionByTag[$_whichOneof(0)];
  void clearOption() => clearField($_whichOneof(0));

  @$pb.TagNumber(1)
  StartTransactionRequest get startTransactionRequest => $_getN(0);
  @$pb.TagNumber(1)
  set startTransactionRequest(StartTransactionRequest v) { setField(1, v); }
  @$pb.TagNumber(1)
  $core.bool hasStartTransactionRequest() => $_has(0);
  @$pb.TagNumber(1)
  void clearStartTransactionRequest() => clearField(1);
  @$pb.TagNumber(1)
  StartTransactionRequest ensureStartTransactionRequest() => $_ensure(0);

  @$pb.TagNumber(2)
  ModelHasRequest get modelHasRequest => $_getN(1);
  @$pb.TagNumber(2)
  set modelHasRequest(ModelHasRequest v) { setField(2, v); }
  @$pb.TagNumber(2)
  $core.bool hasModelHasRequest() => $_has(1);
  @$pb.TagNumber(2)
  void clearModelHasRequest() => clearField(2);
  @$pb.TagNumber(2)
  ModelHasRequest ensureModelHasRequest() => $_ensure(1);

  @$pb.TagNumber(3)
  ModelFindRequest get modelFindRequest => $_getN(2);
  @$pb.TagNumber(3)
  set modelFindRequest(ModelFindRequest v) { setField(3, v); }
  @$pb.TagNumber(3)
  $core.bool hasModelFindRequest() => $_has(2);
  @$pb.TagNumber(3)
  void clearModelFindRequest() => clearField(3);
  @$pb.TagNumber(3)
  ModelFindRequest ensureModelFindRequest() => $_ensure(2);

  @$pb.TagNumber(4)
  ModelFindByIDRequest get modelFindByIDRequest => $_getN(3);
  @$pb.TagNumber(4)
  set modelFindByIDRequest(ModelFindByIDRequest v) { setField(4, v); }
  @$pb.TagNumber(4)
  $core.bool hasModelFindByIDRequest() => $_has(3);
  @$pb.TagNumber(4)
  void clearModelFindByIDRequest() => clearField(4);
  @$pb.TagNumber(4)
  ModelFindByIDRequest ensureModelFindByIDRequest() => $_ensure(3);
}

enum ReadTransactionReply_Option {
  modelHasReply, 
  modelFindReply, 
  modelFindByIDReply, 
  notSet
}

class ReadTransactionReply extends $pb.GeneratedMessage {
  static const $core.Map<$core.int, ReadTransactionReply_Option> _ReadTransactionReply_OptionByTag = {
    1 : ReadTransactionReply_Option.modelHasReply,
    2 : ReadTransactionReply_Option.modelFindReply,
    3 : ReadTransactionReply_Option.modelFindByIDReply,
    0 : ReadTransactionReply_Option.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo('ReadTransactionReply', package: const $pb.PackageName('api.pb'), createEmptyInstance: create)
    ..oo(0, [1, 2, 3])
    ..aOM<ModelHasReply>(1, 'modelHasReply', protoName: 'modelHasReply', subBuilder: ModelHasReply.create)
    ..aOM<ModelFindReply>(2, 'modelFindReply', protoName: 'modelFindReply', subBuilder: ModelFindReply.create)
    ..aOM<ModelFindByIDReply>(3, 'modelFindByIDReply', protoName: 'modelFindByIDReply', subBuilder: ModelFindByIDReply.create)
    ..hasRequiredFields = false
  ;

  ReadTransactionReply._() : super();
  factory ReadTransactionReply() => create();
  factory ReadTransactionReply.fromBuffer($core.List<$core.int> i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromBuffer(i, r);
  factory ReadTransactionReply.fromJson($core.String i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromJson(i, r);
  ReadTransactionReply clone() => ReadTransactionReply()..mergeFromMessage(this);
  ReadTransactionReply copyWith(void Function(ReadTransactionReply) updates) => super.copyWith((message) => updates(message as ReadTransactionReply));
  $pb.BuilderInfo get info_ => _i;
  @$core.pragma('dart2js:noInline')
  static ReadTransactionReply create() => ReadTransactionReply._();
  ReadTransactionReply createEmptyInstance() => create();
  static $pb.PbList<ReadTransactionReply> createRepeated() => $pb.PbList<ReadTransactionReply>();
  @$core.pragma('dart2js:noInline')
  static ReadTransactionReply getDefault() => _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ReadTransactionReply>(create);
  static ReadTransactionReply _defaultInstance;

  ReadTransactionReply_Option whichOption() => _ReadTransactionReply_OptionByTag[$_whichOneof(0)];
  void clearOption() => clearField($_whichOneof(0));

  @$pb.TagNumber(1)
  ModelHasReply get modelHasReply => $_getN(0);
  @$pb.TagNumber(1)
  set modelHasReply(ModelHasReply v) { setField(1, v); }
  @$pb.TagNumber(1)
  $core.bool hasModelHasReply() => $_has(0);
  @$pb.TagNumber(1)
  void clearModelHasReply() => clearField(1);
  @$pb.TagNumber(1)
  ModelHasReply ensureModelHasReply() => $_ensure(0);

  @$pb.TagNumber(2)
  ModelFindReply get modelFindReply => $_getN(1);
  @$pb.TagNumber(2)
  set modelFindReply(ModelFindReply v) { setField(2, v); }
  @$pb.TagNumber(2)
  $core.bool hasModelFindReply() => $_has(1);
  @$pb.TagNumber(2)
  void clearModelFindReply() => clearField(2);
  @$pb.TagNumber(2)
  ModelFindReply ensureModelFindReply() => $_ensure(1);

  @$pb.TagNumber(3)
  ModelFindByIDReply get modelFindByIDReply => $_getN(2);
  @$pb.TagNumber(3)
  set modelFindByIDReply(ModelFindByIDReply v) { setField(3, v); }
  @$pb.TagNumber(3)
  $core.bool hasModelFindByIDReply() => $_has(2);
  @$pb.TagNumber(3)
  void clearModelFindByIDReply() => clearField(3);
  @$pb.TagNumber(3)
  ModelFindByIDReply ensureModelFindByIDReply() => $_ensure(2);
}

enum WriteTransactionRequest_Option {
  startTransactionRequest, 
  modelCreateRequest, 
  modelSaveRequest, 
  modelDeleteRequest, 
  modelHasRequest, 
  modelFindRequest, 
  modelFindByIDRequest, 
  notSet
}

class WriteTransactionRequest extends $pb.GeneratedMessage {
  static const $core.Map<$core.int, WriteTransactionRequest_Option> _WriteTransactionRequest_OptionByTag = {
    1 : WriteTransactionRequest_Option.startTransactionRequest,
    2 : WriteTransactionRequest_Option.modelCreateRequest,
    3 : WriteTransactionRequest_Option.modelSaveRequest,
    4 : WriteTransactionRequest_Option.modelDeleteRequest,
    5 : WriteTransactionRequest_Option.modelHasRequest,
    6 : WriteTransactionRequest_Option.modelFindRequest,
    7 : WriteTransactionRequest_Option.modelFindByIDRequest,
    0 : WriteTransactionRequest_Option.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo('WriteTransactionRequest', package: const $pb.PackageName('api.pb'), createEmptyInstance: create)
    ..oo(0, [1, 2, 3, 4, 5, 6, 7])
    ..aOM<StartTransactionRequest>(1, 'startTransactionRequest', protoName: 'startTransactionRequest', subBuilder: StartTransactionRequest.create)
    ..aOM<ModelCreateRequest>(2, 'modelCreateRequest', protoName: 'modelCreateRequest', subBuilder: ModelCreateRequest.create)
    ..aOM<ModelSaveRequest>(3, 'modelSaveRequest', protoName: 'modelSaveRequest', subBuilder: ModelSaveRequest.create)
    ..aOM<ModelDeleteRequest>(4, 'modelDeleteRequest', protoName: 'modelDeleteRequest', subBuilder: ModelDeleteRequest.create)
    ..aOM<ModelHasRequest>(5, 'modelHasRequest', protoName: 'modelHasRequest', subBuilder: ModelHasRequest.create)
    ..aOM<ModelFindRequest>(6, 'modelFindRequest', protoName: 'modelFindRequest', subBuilder: ModelFindRequest.create)
    ..aOM<ModelFindByIDRequest>(7, 'modelFindByIDRequest', protoName: 'modelFindByIDRequest', subBuilder: ModelFindByIDRequest.create)
    ..hasRequiredFields = false
  ;

  WriteTransactionRequest._() : super();
  factory WriteTransactionRequest() => create();
  factory WriteTransactionRequest.fromBuffer($core.List<$core.int> i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromBuffer(i, r);
  factory WriteTransactionRequest.fromJson($core.String i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromJson(i, r);
  WriteTransactionRequest clone() => WriteTransactionRequest()..mergeFromMessage(this);
  WriteTransactionRequest copyWith(void Function(WriteTransactionRequest) updates) => super.copyWith((message) => updates(message as WriteTransactionRequest));
  $pb.BuilderInfo get info_ => _i;
  @$core.pragma('dart2js:noInline')
  static WriteTransactionRequest create() => WriteTransactionRequest._();
  WriteTransactionRequest createEmptyInstance() => create();
  static $pb.PbList<WriteTransactionRequest> createRepeated() => $pb.PbList<WriteTransactionRequest>();
  @$core.pragma('dart2js:noInline')
  static WriteTransactionRequest getDefault() => _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<WriteTransactionRequest>(create);
  static WriteTransactionRequest _defaultInstance;

  WriteTransactionRequest_Option whichOption() => _WriteTransactionRequest_OptionByTag[$_whichOneof(0)];
  void clearOption() => clearField($_whichOneof(0));

  @$pb.TagNumber(1)
  StartTransactionRequest get startTransactionRequest => $_getN(0);
  @$pb.TagNumber(1)
  set startTransactionRequest(StartTransactionRequest v) { setField(1, v); }
  @$pb.TagNumber(1)
  $core.bool hasStartTransactionRequest() => $_has(0);
  @$pb.TagNumber(1)
  void clearStartTransactionRequest() => clearField(1);
  @$pb.TagNumber(1)
  StartTransactionRequest ensureStartTransactionRequest() => $_ensure(0);

  @$pb.TagNumber(2)
  ModelCreateRequest get modelCreateRequest => $_getN(1);
  @$pb.TagNumber(2)
  set modelCreateRequest(ModelCreateRequest v) { setField(2, v); }
  @$pb.TagNumber(2)
  $core.bool hasModelCreateRequest() => $_has(1);
  @$pb.TagNumber(2)
  void clearModelCreateRequest() => clearField(2);
  @$pb.TagNumber(2)
  ModelCreateRequest ensureModelCreateRequest() => $_ensure(1);

  @$pb.TagNumber(3)
  ModelSaveRequest get modelSaveRequest => $_getN(2);
  @$pb.TagNumber(3)
  set modelSaveRequest(ModelSaveRequest v) { setField(3, v); }
  @$pb.TagNumber(3)
  $core.bool hasModelSaveRequest() => $_has(2);
  @$pb.TagNumber(3)
  void clearModelSaveRequest() => clearField(3);
  @$pb.TagNumber(3)
  ModelSaveRequest ensureModelSaveRequest() => $_ensure(2);

  @$pb.TagNumber(4)
  ModelDeleteRequest get modelDeleteRequest => $_getN(3);
  @$pb.TagNumber(4)
  set modelDeleteRequest(ModelDeleteRequest v) { setField(4, v); }
  @$pb.TagNumber(4)
  $core.bool hasModelDeleteRequest() => $_has(3);
  @$pb.TagNumber(4)
  void clearModelDeleteRequest() => clearField(4);
  @$pb.TagNumber(4)
  ModelDeleteRequest ensureModelDeleteRequest() => $_ensure(3);

  @$pb.TagNumber(5)
  ModelHasRequest get modelHasRequest => $_getN(4);
  @$pb.TagNumber(5)
  set modelHasRequest(ModelHasRequest v) { setField(5, v); }
  @$pb.TagNumber(5)
  $core.bool hasModelHasRequest() => $_has(4);
  @$pb.TagNumber(5)
  void clearModelHasRequest() => clearField(5);
  @$pb.TagNumber(5)
  ModelHasRequest ensureModelHasRequest() => $_ensure(4);

  @$pb.TagNumber(6)
  ModelFindRequest get modelFindRequest => $_getN(5);
  @$pb.TagNumber(6)
  set modelFindRequest(ModelFindRequest v) { setField(6, v); }
  @$pb.TagNumber(6)
  $core.bool hasModelFindRequest() => $_has(5);
  @$pb.TagNumber(6)
  void clearModelFindRequest() => clearField(6);
  @$pb.TagNumber(6)
  ModelFindRequest ensureModelFindRequest() => $_ensure(5);

  @$pb.TagNumber(7)
  ModelFindByIDRequest get modelFindByIDRequest => $_getN(6);
  @$pb.TagNumber(7)
  set modelFindByIDRequest(ModelFindByIDRequest v) { setField(7, v); }
  @$pb.TagNumber(7)
  $core.bool hasModelFindByIDRequest() => $_has(6);
  @$pb.TagNumber(7)
  void clearModelFindByIDRequest() => clearField(7);
  @$pb.TagNumber(7)
  ModelFindByIDRequest ensureModelFindByIDRequest() => $_ensure(6);
}

enum WriteTransactionReply_Option {
  modelCreateReply, 
  modelSaveReply, 
  modelDeleteReply, 
  modelHasReply, 
  modelFindReply, 
  modelFindByIDReply, 
  notSet
}

class WriteTransactionReply extends $pb.GeneratedMessage {
  static const $core.Map<$core.int, WriteTransactionReply_Option> _WriteTransactionReply_OptionByTag = {
    1 : WriteTransactionReply_Option.modelCreateReply,
    2 : WriteTransactionReply_Option.modelSaveReply,
    3 : WriteTransactionReply_Option.modelDeleteReply,
    4 : WriteTransactionReply_Option.modelHasReply,
    5 : WriteTransactionReply_Option.modelFindReply,
    6 : WriteTransactionReply_Option.modelFindByIDReply,
    0 : WriteTransactionReply_Option.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo('WriteTransactionReply', package: const $pb.PackageName('api.pb'), createEmptyInstance: create)
    ..oo(0, [1, 2, 3, 4, 5, 6])
    ..aOM<ModelCreateReply>(1, 'modelCreateReply', protoName: 'modelCreateReply', subBuilder: ModelCreateReply.create)
    ..aOM<ModelSaveReply>(2, 'modelSaveReply', protoName: 'modelSaveReply', subBuilder: ModelSaveReply.create)
    ..aOM<ModelDeleteReply>(3, 'modelDeleteReply', protoName: 'modelDeleteReply', subBuilder: ModelDeleteReply.create)
    ..aOM<ModelHasReply>(4, 'modelHasReply', protoName: 'modelHasReply', subBuilder: ModelHasReply.create)
    ..aOM<ModelFindReply>(5, 'modelFindReply', protoName: 'modelFindReply', subBuilder: ModelFindReply.create)
    ..aOM<ModelFindByIDReply>(6, 'modelFindByIDReply', protoName: 'modelFindByIDReply', subBuilder: ModelFindByIDReply.create)
    ..hasRequiredFields = false
  ;

  WriteTransactionReply._() : super();
  factory WriteTransactionReply() => create();
  factory WriteTransactionReply.fromBuffer($core.List<$core.int> i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromBuffer(i, r);
  factory WriteTransactionReply.fromJson($core.String i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromJson(i, r);
  WriteTransactionReply clone() => WriteTransactionReply()..mergeFromMessage(this);
  WriteTransactionReply copyWith(void Function(WriteTransactionReply) updates) => super.copyWith((message) => updates(message as WriteTransactionReply));
  $pb.BuilderInfo get info_ => _i;
  @$core.pragma('dart2js:noInline')
  static WriteTransactionReply create() => WriteTransactionReply._();
  WriteTransactionReply createEmptyInstance() => create();
  static $pb.PbList<WriteTransactionReply> createRepeated() => $pb.PbList<WriteTransactionReply>();
  @$core.pragma('dart2js:noInline')
  static WriteTransactionReply getDefault() => _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<WriteTransactionReply>(create);
  static WriteTransactionReply _defaultInstance;

  WriteTransactionReply_Option whichOption() => _WriteTransactionReply_OptionByTag[$_whichOneof(0)];
  void clearOption() => clearField($_whichOneof(0));

  @$pb.TagNumber(1)
  ModelCreateReply get modelCreateReply => $_getN(0);
  @$pb.TagNumber(1)
  set modelCreateReply(ModelCreateReply v) { setField(1, v); }
  @$pb.TagNumber(1)
  $core.bool hasModelCreateReply() => $_has(0);
  @$pb.TagNumber(1)
  void clearModelCreateReply() => clearField(1);
  @$pb.TagNumber(1)
  ModelCreateReply ensureModelCreateReply() => $_ensure(0);

  @$pb.TagNumber(2)
  ModelSaveReply get modelSaveReply => $_getN(1);
  @$pb.TagNumber(2)
  set modelSaveReply(ModelSaveReply v) { setField(2, v); }
  @$pb.TagNumber(2)
  $core.bool hasModelSaveReply() => $_has(1);
  @$pb.TagNumber(2)
  void clearModelSaveReply() => clearField(2);
  @$pb.TagNumber(2)
  ModelSaveReply ensureModelSaveReply() => $_ensure(1);

  @$pb.TagNumber(3)
  ModelDeleteReply get modelDeleteReply => $_getN(2);
  @$pb.TagNumber(3)
  set modelDeleteReply(ModelDeleteReply v) { setField(3, v); }
  @$pb.TagNumber(3)
  $core.bool hasModelDeleteReply() => $_has(2);
  @$pb.TagNumber(3)
  void clearModelDeleteReply() => clearField(3);
  @$pb.TagNumber(3)
  ModelDeleteReply ensureModelDeleteReply() => $_ensure(2);

  @$pb.TagNumber(4)
  ModelHasReply get modelHasReply => $_getN(3);
  @$pb.TagNumber(4)
  set modelHasReply(ModelHasReply v) { setField(4, v); }
  @$pb.TagNumber(4)
  $core.bool hasModelHasReply() => $_has(3);
  @$pb.TagNumber(4)
  void clearModelHasReply() => clearField(4);
  @$pb.TagNumber(4)
  ModelHasReply ensureModelHasReply() => $_ensure(3);

  @$pb.TagNumber(5)
  ModelFindReply get modelFindReply => $_getN(4);
  @$pb.TagNumber(5)
  set modelFindReply(ModelFindReply v) { setField(5, v); }
  @$pb.TagNumber(5)
  $core.bool hasModelFindReply() => $_has(4);
  @$pb.TagNumber(5)
  void clearModelFindReply() => clearField(5);
  @$pb.TagNumber(5)
  ModelFindReply ensureModelFindReply() => $_ensure(4);

  @$pb.TagNumber(6)
  ModelFindByIDReply get modelFindByIDReply => $_getN(5);
  @$pb.TagNumber(6)
  set modelFindByIDReply(ModelFindByIDReply v) { setField(6, v); }
  @$pb.TagNumber(6)
  $core.bool hasModelFindByIDReply() => $_has(5);
  @$pb.TagNumber(6)
  void clearModelFindByIDReply() => clearField(6);
  @$pb.TagNumber(6)
  ModelFindByIDReply ensureModelFindByIDReply() => $_ensure(5);
}

class ListenRequest_Filter extends $pb.GeneratedMessage {
  static final $pb.BuilderInfo _i = $pb.BuilderInfo('ListenRequest.Filter', package: const $pb.PackageName('api.pb'), createEmptyInstance: create)
    ..aOS(1, 'modelName', protoName: 'modelName')
    ..aOS(2, 'entityID', protoName: 'entityID')
    ..e<ListenRequest_Filter_Action>(3, 'action', $pb.PbFieldType.OE, defaultOrMaker: ListenRequest_Filter_Action.ALL, valueOf: ListenRequest_Filter_Action.valueOf, enumValues: ListenRequest_Filter_Action.values)
    ..hasRequiredFields = false
  ;

  ListenRequest_Filter._() : super();
  factory ListenRequest_Filter() => create();
  factory ListenRequest_Filter.fromBuffer($core.List<$core.int> i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromBuffer(i, r);
  factory ListenRequest_Filter.fromJson($core.String i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromJson(i, r);
  ListenRequest_Filter clone() => ListenRequest_Filter()..mergeFromMessage(this);
  ListenRequest_Filter copyWith(void Function(ListenRequest_Filter) updates) => super.copyWith((message) => updates(message as ListenRequest_Filter));
  $pb.BuilderInfo get info_ => _i;
  @$core.pragma('dart2js:noInline')
  static ListenRequest_Filter create() => ListenRequest_Filter._();
  ListenRequest_Filter createEmptyInstance() => create();
  static $pb.PbList<ListenRequest_Filter> createRepeated() => $pb.PbList<ListenRequest_Filter>();
  @$core.pragma('dart2js:noInline')
  static ListenRequest_Filter getDefault() => _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ListenRequest_Filter>(create);
  static ListenRequest_Filter _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get modelName => $_getSZ(0);
  @$pb.TagNumber(1)
  set modelName($core.String v) { $_setString(0, v); }
  @$pb.TagNumber(1)
  $core.bool hasModelName() => $_has(0);
  @$pb.TagNumber(1)
  void clearModelName() => clearField(1);

  @$pb.TagNumber(2)
  $core.String get entityID => $_getSZ(1);
  @$pb.TagNumber(2)
  set entityID($core.String v) { $_setString(1, v); }
  @$pb.TagNumber(2)
  $core.bool hasEntityID() => $_has(1);
  @$pb.TagNumber(2)
  void clearEntityID() => clearField(2);

  @$pb.TagNumber(3)
  ListenRequest_Filter_Action get action => $_getN(2);
  @$pb.TagNumber(3)
  set action(ListenRequest_Filter_Action v) { setField(3, v); }
  @$pb.TagNumber(3)
  $core.bool hasAction() => $_has(2);
  @$pb.TagNumber(3)
  void clearAction() => clearField(3);
}

class ListenRequest extends $pb.GeneratedMessage {
  static final $pb.BuilderInfo _i = $pb.BuilderInfo('ListenRequest', package: const $pb.PackageName('api.pb'), createEmptyInstance: create)
    ..aOS(1, 'storeID', protoName: 'storeID')
    ..pc<ListenRequest_Filter>(2, 'filters', $pb.PbFieldType.PM, subBuilder: ListenRequest_Filter.create)
    ..hasRequiredFields = false
  ;

  ListenRequest._() : super();
  factory ListenRequest() => create();
  factory ListenRequest.fromBuffer($core.List<$core.int> i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromBuffer(i, r);
  factory ListenRequest.fromJson($core.String i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromJson(i, r);
  ListenRequest clone() => ListenRequest()..mergeFromMessage(this);
  ListenRequest copyWith(void Function(ListenRequest) updates) => super.copyWith((message) => updates(message as ListenRequest));
  $pb.BuilderInfo get info_ => _i;
  @$core.pragma('dart2js:noInline')
  static ListenRequest create() => ListenRequest._();
  ListenRequest createEmptyInstance() => create();
  static $pb.PbList<ListenRequest> createRepeated() => $pb.PbList<ListenRequest>();
  @$core.pragma('dart2js:noInline')
  static ListenRequest getDefault() => _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ListenRequest>(create);
  static ListenRequest _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get storeID => $_getSZ(0);
  @$pb.TagNumber(1)
  set storeID($core.String v) { $_setString(0, v); }
  @$pb.TagNumber(1)
  $core.bool hasStoreID() => $_has(0);
  @$pb.TagNumber(1)
  void clearStoreID() => clearField(1);

  @$pb.TagNumber(2)
  $core.List<ListenRequest_Filter> get filters => $_getList(1);
}

class ListenReply extends $pb.GeneratedMessage {
  static final $pb.BuilderInfo _i = $pb.BuilderInfo('ListenReply', package: const $pb.PackageName('api.pb'), createEmptyInstance: create)
    ..aOS(1, 'modelName', protoName: 'modelName')
    ..aOS(2, 'entityID', protoName: 'entityID')
    ..e<ListenReply_Action>(3, 'action', $pb.PbFieldType.OE, defaultOrMaker: ListenReply_Action.CREATE, valueOf: ListenReply_Action.valueOf, enumValues: ListenReply_Action.values)
    ..a<$core.List<$core.int>>(4, 'entity', $pb.PbFieldType.OY)
    ..hasRequiredFields = false
  ;

  ListenReply._() : super();
  factory ListenReply() => create();
  factory ListenReply.fromBuffer($core.List<$core.int> i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromBuffer(i, r);
  factory ListenReply.fromJson($core.String i, [$pb.ExtensionRegistry r = $pb.ExtensionRegistry.EMPTY]) => create()..mergeFromJson(i, r);
  ListenReply clone() => ListenReply()..mergeFromMessage(this);
  ListenReply copyWith(void Function(ListenReply) updates) => super.copyWith((message) => updates(message as ListenReply));
  $pb.BuilderInfo get info_ => _i;
  @$core.pragma('dart2js:noInline')
  static ListenReply create() => ListenReply._();
  ListenReply createEmptyInstance() => create();
  static $pb.PbList<ListenReply> createRepeated() => $pb.PbList<ListenReply>();
  @$core.pragma('dart2js:noInline')
  static ListenReply getDefault() => _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ListenReply>(create);
  static ListenReply _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get modelName => $_getSZ(0);
  @$pb.TagNumber(1)
  set modelName($core.String v) { $_setString(0, v); }
  @$pb.TagNumber(1)
  $core.bool hasModelName() => $_has(0);
  @$pb.TagNumber(1)
  void clearModelName() => clearField(1);

  @$pb.TagNumber(2)
  $core.String get entityID => $_getSZ(1);
  @$pb.TagNumber(2)
  set entityID($core.String v) { $_setString(1, v); }
  @$pb.TagNumber(2)
  $core.bool hasEntityID() => $_has(1);
  @$pb.TagNumber(2)
  void clearEntityID() => clearField(2);

  @$pb.TagNumber(3)
  ListenReply_Action get action => $_getN(2);
  @$pb.TagNumber(3)
  set action(ListenReply_Action v) { setField(3, v); }
  @$pb.TagNumber(3)
  $core.bool hasAction() => $_has(2);
  @$pb.TagNumber(3)
  void clearAction() => clearField(3);

  @$pb.TagNumber(4)
  $core.List<$core.int> get entity => $_getN(3);
  @$pb.TagNumber(4)
  set entity($core.List<$core.int> v) { $_setBytes(3, v); }
  @$pb.TagNumber(4)
  $core.bool hasEntity() => $_has(3);
  @$pb.TagNumber(4)
  void clearEntity() => clearField(4);
}

