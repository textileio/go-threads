///
//  Generated code. Do not modify.
//  source: api.proto
//
// @dart = 2.3
// ignore_for_file: camel_case_types,non_constant_identifier_names,library_prefixes,unused_import,unused_shown_name,return_of_invalid_type

import 'dart:async' as $async;

import 'dart:core' as $core;

import 'package:grpc/service_api.dart' as $grpc;
import 'api.pb.dart' as $0;
export 'api.pb.dart';

class APIClient extends $grpc.Client {
  static final _$newStore =
      $grpc.ClientMethod<$0.NewStoreRequest, $0.NewStoreReply>(
          '/api.pb.API/NewStore',
          ($0.NewStoreRequest value) => value.writeToBuffer(),
          ($core.List<$core.int> value) => $0.NewStoreReply.fromBuffer(value));
  static final _$registerSchema =
      $grpc.ClientMethod<$0.RegisterSchemaRequest, $0.RegisterSchemaReply>(
          '/api.pb.API/RegisterSchema',
          ($0.RegisterSchemaRequest value) => value.writeToBuffer(),
          ($core.List<$core.int> value) =>
              $0.RegisterSchemaReply.fromBuffer(value));
  static final _$start = $grpc.ClientMethod<$0.StartRequest, $0.StartReply>(
      '/api.pb.API/Start',
      ($0.StartRequest value) => value.writeToBuffer(),
      ($core.List<$core.int> value) => $0.StartReply.fromBuffer(value));
  static final _$startFromAddress =
      $grpc.ClientMethod<$0.StartFromAddressRequest, $0.StartFromAddressReply>(
          '/api.pb.API/StartFromAddress',
          ($0.StartFromAddressRequest value) => value.writeToBuffer(),
          ($core.List<$core.int> value) =>
              $0.StartFromAddressReply.fromBuffer(value));
  static final _$getStoreLink =
      $grpc.ClientMethod<$0.GetStoreLinkRequest, $0.GetStoreLinkReply>(
          '/api.pb.API/GetStoreLink',
          ($0.GetStoreLinkRequest value) => value.writeToBuffer(),
          ($core.List<$core.int> value) =>
              $0.GetStoreLinkReply.fromBuffer(value));
  static final _$modelCreate =
      $grpc.ClientMethod<$0.ModelCreateRequest, $0.ModelCreateReply>(
          '/api.pb.API/ModelCreate',
          ($0.ModelCreateRequest value) => value.writeToBuffer(),
          ($core.List<$core.int> value) =>
              $0.ModelCreateReply.fromBuffer(value));
  static final _$modelSave =
      $grpc.ClientMethod<$0.ModelSaveRequest, $0.ModelSaveReply>(
          '/api.pb.API/ModelSave',
          ($0.ModelSaveRequest value) => value.writeToBuffer(),
          ($core.List<$core.int> value) => $0.ModelSaveReply.fromBuffer(value));
  static final _$modelDelete =
      $grpc.ClientMethod<$0.ModelDeleteRequest, $0.ModelDeleteReply>(
          '/api.pb.API/ModelDelete',
          ($0.ModelDeleteRequest value) => value.writeToBuffer(),
          ($core.List<$core.int> value) =>
              $0.ModelDeleteReply.fromBuffer(value));
  static final _$modelHas =
      $grpc.ClientMethod<$0.ModelHasRequest, $0.ModelHasReply>(
          '/api.pb.API/ModelHas',
          ($0.ModelHasRequest value) => value.writeToBuffer(),
          ($core.List<$core.int> value) => $0.ModelHasReply.fromBuffer(value));
  static final _$modelFind =
      $grpc.ClientMethod<$0.ModelFindRequest, $0.ModelFindReply>(
          '/api.pb.API/ModelFind',
          ($0.ModelFindRequest value) => value.writeToBuffer(),
          ($core.List<$core.int> value) => $0.ModelFindReply.fromBuffer(value));
  static final _$modelFindByID =
      $grpc.ClientMethod<$0.ModelFindByIDRequest, $0.ModelFindByIDReply>(
          '/api.pb.API/ModelFindByID',
          ($0.ModelFindByIDRequest value) => value.writeToBuffer(),
          ($core.List<$core.int> value) =>
              $0.ModelFindByIDReply.fromBuffer(value));
  static final _$readTransaction =
      $grpc.ClientMethod<$0.ReadTransactionRequest, $0.ReadTransactionReply>(
          '/api.pb.API/ReadTransaction',
          ($0.ReadTransactionRequest value) => value.writeToBuffer(),
          ($core.List<$core.int> value) =>
              $0.ReadTransactionReply.fromBuffer(value));
  static final _$writeTransaction =
      $grpc.ClientMethod<$0.WriteTransactionRequest, $0.WriteTransactionReply>(
          '/api.pb.API/WriteTransaction',
          ($0.WriteTransactionRequest value) => value.writeToBuffer(),
          ($core.List<$core.int> value) =>
              $0.WriteTransactionReply.fromBuffer(value));
  static final _$listen = $grpc.ClientMethod<$0.ListenRequest, $0.ListenReply>(
      '/api.pb.API/Listen',
      ($0.ListenRequest value) => value.writeToBuffer(),
      ($core.List<$core.int> value) => $0.ListenReply.fromBuffer(value));

  APIClient($grpc.ClientChannel channel, {$grpc.CallOptions options})
      : super(channel, options: options);

  $grpc.ResponseFuture<$0.NewStoreReply> newStore($0.NewStoreRequest request,
      {$grpc.CallOptions options}) {
    final call = $createCall(_$newStore, $async.Stream.fromIterable([request]),
        options: options);
    return $grpc.ResponseFuture(call);
  }

  $grpc.ResponseFuture<$0.RegisterSchemaReply> registerSchema(
      $0.RegisterSchemaRequest request,
      {$grpc.CallOptions options}) {
    final call = $createCall(
        _$registerSchema, $async.Stream.fromIterable([request]),
        options: options);
    return $grpc.ResponseFuture(call);
  }

  $grpc.ResponseFuture<$0.StartReply> start($0.StartRequest request,
      {$grpc.CallOptions options}) {
    final call = $createCall(_$start, $async.Stream.fromIterable([request]),
        options: options);
    return $grpc.ResponseFuture(call);
  }

  $grpc.ResponseFuture<$0.StartFromAddressReply> startFromAddress(
      $0.StartFromAddressRequest request,
      {$grpc.CallOptions options}) {
    final call = $createCall(
        _$startFromAddress, $async.Stream.fromIterable([request]),
        options: options);
    return $grpc.ResponseFuture(call);
  }

  $grpc.ResponseFuture<$0.GetStoreLinkReply> getStoreLink(
      $0.GetStoreLinkRequest request,
      {$grpc.CallOptions options}) {
    final call = $createCall(
        _$getStoreLink, $async.Stream.fromIterable([request]),
        options: options);
    return $grpc.ResponseFuture(call);
  }

  $grpc.ResponseFuture<$0.ModelCreateReply> modelCreate(
      $0.ModelCreateRequest request,
      {$grpc.CallOptions options}) {
    final call = $createCall(
        _$modelCreate, $async.Stream.fromIterable([request]),
        options: options);
    return $grpc.ResponseFuture(call);
  }

  $grpc.ResponseFuture<$0.ModelSaveReply> modelSave($0.ModelSaveRequest request,
      {$grpc.CallOptions options}) {
    final call = $createCall(_$modelSave, $async.Stream.fromIterable([request]),
        options: options);
    return $grpc.ResponseFuture(call);
  }

  $grpc.ResponseFuture<$0.ModelDeleteReply> modelDelete(
      $0.ModelDeleteRequest request,
      {$grpc.CallOptions options}) {
    final call = $createCall(
        _$modelDelete, $async.Stream.fromIterable([request]),
        options: options);
    return $grpc.ResponseFuture(call);
  }

  $grpc.ResponseFuture<$0.ModelHasReply> modelHas($0.ModelHasRequest request,
      {$grpc.CallOptions options}) {
    final call = $createCall(_$modelHas, $async.Stream.fromIterable([request]),
        options: options);
    return $grpc.ResponseFuture(call);
  }

  $grpc.ResponseFuture<$0.ModelFindReply> modelFind($0.ModelFindRequest request,
      {$grpc.CallOptions options}) {
    final call = $createCall(_$modelFind, $async.Stream.fromIterable([request]),
        options: options);
    return $grpc.ResponseFuture(call);
  }

  $grpc.ResponseFuture<$0.ModelFindByIDReply> modelFindByID(
      $0.ModelFindByIDRequest request,
      {$grpc.CallOptions options}) {
    final call = $createCall(
        _$modelFindByID, $async.Stream.fromIterable([request]),
        options: options);
    return $grpc.ResponseFuture(call);
  }

  $grpc.ResponseStream<$0.ReadTransactionReply> readTransaction(
      $async.Stream<$0.ReadTransactionRequest> request,
      {$grpc.CallOptions options}) {
    final call = $createCall(_$readTransaction, request, options: options);
    return $grpc.ResponseStream(call);
  }

  $grpc.ResponseStream<$0.WriteTransactionReply> writeTransaction(
      $async.Stream<$0.WriteTransactionRequest> request,
      {$grpc.CallOptions options}) {
    final call = $createCall(_$writeTransaction, request, options: options);
    return $grpc.ResponseStream(call);
  }

  $grpc.ResponseStream<$0.ListenReply> listen($0.ListenRequest request,
      {$grpc.CallOptions options}) {
    final call = $createCall(_$listen, $async.Stream.fromIterable([request]),
        options: options);
    return $grpc.ResponseStream(call);
  }
}

abstract class APIServiceBase extends $grpc.Service {
  $core.String get $name => 'api.pb.API';

  APIServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.NewStoreRequest, $0.NewStoreReply>(
        'NewStore',
        newStore_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.NewStoreRequest.fromBuffer(value),
        ($0.NewStoreReply value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.RegisterSchemaRequest, $0.RegisterSchemaReply>(
            'RegisterSchema',
            registerSchema_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.RegisterSchemaRequest.fromBuffer(value),
            ($0.RegisterSchemaReply value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.StartRequest, $0.StartReply>(
        'Start',
        start_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.StartRequest.fromBuffer(value),
        ($0.StartReply value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.StartFromAddressRequest,
            $0.StartFromAddressReply>(
        'StartFromAddress',
        startFromAddress_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.StartFromAddressRequest.fromBuffer(value),
        ($0.StartFromAddressReply value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.GetStoreLinkRequest, $0.GetStoreLinkReply>(
            'GetStoreLink',
            getStoreLink_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.GetStoreLinkRequest.fromBuffer(value),
            ($0.GetStoreLinkReply value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ModelCreateRequest, $0.ModelCreateReply>(
        'ModelCreate',
        modelCreate_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ModelCreateRequest.fromBuffer(value),
        ($0.ModelCreateReply value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ModelSaveRequest, $0.ModelSaveReply>(
        'ModelSave',
        modelSave_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.ModelSaveRequest.fromBuffer(value),
        ($0.ModelSaveReply value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ModelDeleteRequest, $0.ModelDeleteReply>(
        'ModelDelete',
        modelDelete_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ModelDeleteRequest.fromBuffer(value),
        ($0.ModelDeleteReply value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ModelHasRequest, $0.ModelHasReply>(
        'ModelHas',
        modelHas_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.ModelHasRequest.fromBuffer(value),
        ($0.ModelHasReply value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ModelFindRequest, $0.ModelFindReply>(
        'ModelFind',
        modelFind_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.ModelFindRequest.fromBuffer(value),
        ($0.ModelFindReply value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.ModelFindByIDRequest, $0.ModelFindByIDReply>(
            'ModelFindByID',
            modelFindByID_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.ModelFindByIDRequest.fromBuffer(value),
            ($0.ModelFindByIDReply value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.ReadTransactionRequest, $0.ReadTransactionReply>(
            'ReadTransaction',
            readTransaction,
            true,
            true,
            ($core.List<$core.int> value) =>
                $0.ReadTransactionRequest.fromBuffer(value),
            ($0.ReadTransactionReply value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.WriteTransactionRequest,
            $0.WriteTransactionReply>(
        'WriteTransaction',
        writeTransaction,
        true,
        true,
        ($core.List<$core.int> value) =>
            $0.WriteTransactionRequest.fromBuffer(value),
        ($0.WriteTransactionReply value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ListenRequest, $0.ListenReply>(
        'Listen',
        listen_Pre,
        false,
        true,
        ($core.List<$core.int> value) => $0.ListenRequest.fromBuffer(value),
        ($0.ListenReply value) => value.writeToBuffer()));
  }

  $async.Future<$0.NewStoreReply> newStore_Pre(
      $grpc.ServiceCall call, $async.Future<$0.NewStoreRequest> request) async {
    return newStore(call, await request);
  }

  $async.Future<$0.RegisterSchemaReply> registerSchema_Pre(
      $grpc.ServiceCall call,
      $async.Future<$0.RegisterSchemaRequest> request) async {
    return registerSchema(call, await request);
  }

  $async.Future<$0.StartReply> start_Pre(
      $grpc.ServiceCall call, $async.Future<$0.StartRequest> request) async {
    return start(call, await request);
  }

  $async.Future<$0.StartFromAddressReply> startFromAddress_Pre(
      $grpc.ServiceCall call,
      $async.Future<$0.StartFromAddressRequest> request) async {
    return startFromAddress(call, await request);
  }

  $async.Future<$0.GetStoreLinkReply> getStoreLink_Pre($grpc.ServiceCall call,
      $async.Future<$0.GetStoreLinkRequest> request) async {
    return getStoreLink(call, await request);
  }

  $async.Future<$0.ModelCreateReply> modelCreate_Pre($grpc.ServiceCall call,
      $async.Future<$0.ModelCreateRequest> request) async {
    return modelCreate(call, await request);
  }

  $async.Future<$0.ModelSaveReply> modelSave_Pre($grpc.ServiceCall call,
      $async.Future<$0.ModelSaveRequest> request) async {
    return modelSave(call, await request);
  }

  $async.Future<$0.ModelDeleteReply> modelDelete_Pre($grpc.ServiceCall call,
      $async.Future<$0.ModelDeleteRequest> request) async {
    return modelDelete(call, await request);
  }

  $async.Future<$0.ModelHasReply> modelHas_Pre(
      $grpc.ServiceCall call, $async.Future<$0.ModelHasRequest> request) async {
    return modelHas(call, await request);
  }

  $async.Future<$0.ModelFindReply> modelFind_Pre($grpc.ServiceCall call,
      $async.Future<$0.ModelFindRequest> request) async {
    return modelFind(call, await request);
  }

  $async.Future<$0.ModelFindByIDReply> modelFindByID_Pre($grpc.ServiceCall call,
      $async.Future<$0.ModelFindByIDRequest> request) async {
    return modelFindByID(call, await request);
  }

  $async.Stream<$0.ListenReply> listen_Pre(
      $grpc.ServiceCall call, $async.Future<$0.ListenRequest> request) async* {
    yield* listen(call, await request);
  }

  $async.Future<$0.NewStoreReply> newStore(
      $grpc.ServiceCall call, $0.NewStoreRequest request);
  $async.Future<$0.RegisterSchemaReply> registerSchema(
      $grpc.ServiceCall call, $0.RegisterSchemaRequest request);
  $async.Future<$0.StartReply> start(
      $grpc.ServiceCall call, $0.StartRequest request);
  $async.Future<$0.StartFromAddressReply> startFromAddress(
      $grpc.ServiceCall call, $0.StartFromAddressRequest request);
  $async.Future<$0.GetStoreLinkReply> getStoreLink(
      $grpc.ServiceCall call, $0.GetStoreLinkRequest request);
  $async.Future<$0.ModelCreateReply> modelCreate(
      $grpc.ServiceCall call, $0.ModelCreateRequest request);
  $async.Future<$0.ModelSaveReply> modelSave(
      $grpc.ServiceCall call, $0.ModelSaveRequest request);
  $async.Future<$0.ModelDeleteReply> modelDelete(
      $grpc.ServiceCall call, $0.ModelDeleteRequest request);
  $async.Future<$0.ModelHasReply> modelHas(
      $grpc.ServiceCall call, $0.ModelHasRequest request);
  $async.Future<$0.ModelFindReply> modelFind(
      $grpc.ServiceCall call, $0.ModelFindRequest request);
  $async.Future<$0.ModelFindByIDReply> modelFindByID(
      $grpc.ServiceCall call, $0.ModelFindByIDRequest request);
  $async.Stream<$0.ReadTransactionReply> readTransaction(
      $grpc.ServiceCall call, $async.Stream<$0.ReadTransactionRequest> request);
  $async.Stream<$0.WriteTransactionReply> writeTransaction(
      $grpc.ServiceCall call,
      $async.Stream<$0.WriteTransactionRequest> request);
  $async.Stream<$0.ListenReply> listen(
      $grpc.ServiceCall call, $0.ListenRequest request);
}
