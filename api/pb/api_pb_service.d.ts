// package: api.pb
// file: api.proto

import * as api_pb from "./api_pb";
import {grpc} from "@improbable-eng/grpc-web";

type APINewStore = {
  readonly methodName: string;
  readonly service: typeof API;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof api_pb.NewStoreRequest;
  readonly responseType: typeof api_pb.NewStoreReply;
};

type APIRegisterSchema = {
  readonly methodName: string;
  readonly service: typeof API;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof api_pb.RegisterSchemaRequest;
  readonly responseType: typeof api_pb.RegisterSchemaReply;
};

type APIStart = {
  readonly methodName: string;
  readonly service: typeof API;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof api_pb.StartRequest;
  readonly responseType: typeof api_pb.StartReply;
};

type APIStartFromAddress = {
  readonly methodName: string;
  readonly service: typeof API;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof api_pb.StartFromAddressRequest;
  readonly responseType: typeof api_pb.StartFromAddressReply;
};

type APIModelCreate = {
  readonly methodName: string;
  readonly service: typeof API;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof api_pb.ModelCreateRequest;
  readonly responseType: typeof api_pb.ModelCreateReply;
};

type APIModelSave = {
  readonly methodName: string;
  readonly service: typeof API;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof api_pb.ModelSaveRequest;
  readonly responseType: typeof api_pb.ModelSaveReply;
};

type APIModelDelete = {
  readonly methodName: string;
  readonly service: typeof API;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof api_pb.ModelDeleteRequest;
  readonly responseType: typeof api_pb.ModelDeleteReply;
};

type APIModelHas = {
  readonly methodName: string;
  readonly service: typeof API;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof api_pb.ModelHasRequest;
  readonly responseType: typeof api_pb.ModelHasReply;
};

type APIModelFind = {
  readonly methodName: string;
  readonly service: typeof API;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof api_pb.ModelFindRequest;
  readonly responseType: typeof api_pb.ModelFindReply;
};

type APIModelFindByID = {
  readonly methodName: string;
  readonly service: typeof API;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof api_pb.ModelFindByIDRequest;
  readonly responseType: typeof api_pb.ModelFindByIDReply;
};

type APIReadTransaction = {
  readonly methodName: string;
  readonly service: typeof API;
  readonly requestStream: true;
  readonly responseStream: true;
  readonly requestType: typeof api_pb.ReadTransactionRequest;
  readonly responseType: typeof api_pb.ReadTransactionReply;
};

type APIWriteTransaction = {
  readonly methodName: string;
  readonly service: typeof API;
  readonly requestStream: true;
  readonly responseStream: true;
  readonly requestType: typeof api_pb.WriteTransactionRequest;
  readonly responseType: typeof api_pb.WriteTransactionReply;
};

type APIListen = {
  readonly methodName: string;
  readonly service: typeof API;
  readonly requestStream: false;
  readonly responseStream: true;
  readonly requestType: typeof api_pb.ListenRequest;
  readonly responseType: typeof api_pb.ListenReply;
};

export class API {
  static readonly serviceName: string;
  static readonly NewStore: APINewStore;
  static readonly RegisterSchema: APIRegisterSchema;
  static readonly Start: APIStart;
  static readonly StartFromAddress: APIStartFromAddress;
  static readonly ModelCreate: APIModelCreate;
  static readonly ModelSave: APIModelSave;
  static readonly ModelDelete: APIModelDelete;
  static readonly ModelHas: APIModelHas;
  static readonly ModelFind: APIModelFind;
  static readonly ModelFindByID: APIModelFindByID;
  static readonly ReadTransaction: APIReadTransaction;
  static readonly WriteTransaction: APIWriteTransaction;
  static readonly Listen: APIListen;
}

export type ServiceError = { message: string, code: number; metadata: grpc.Metadata }
export type Status = { details: string, code: number; metadata: grpc.Metadata }

interface UnaryResponse {
  cancel(): void;
}
interface ResponseStream<T> {
  cancel(): void;
  on(type: 'data', handler: (message: T) => void): ResponseStream<T>;
  on(type: 'end', handler: (status?: Status) => void): ResponseStream<T>;
  on(type: 'status', handler: (status: Status) => void): ResponseStream<T>;
}
interface RequestStream<T> {
  write(message: T): RequestStream<T>;
  end(): void;
  cancel(): void;
  on(type: 'end', handler: (status?: Status) => void): RequestStream<T>;
  on(type: 'status', handler: (status: Status) => void): RequestStream<T>;
}
interface BidirectionalStream<ReqT, ResT> {
  write(message: ReqT): BidirectionalStream<ReqT, ResT>;
  end(): void;
  cancel(): void;
  on(type: 'data', handler: (message: ResT) => void): BidirectionalStream<ReqT, ResT>;
  on(type: 'end', handler: (status?: Status) => void): BidirectionalStream<ReqT, ResT>;
  on(type: 'status', handler: (status: Status) => void): BidirectionalStream<ReqT, ResT>;
}

export class APIClient {
  readonly serviceHost: string;

  constructor(serviceHost: string, options?: grpc.RpcOptions);
  newStore(
    requestMessage: api_pb.NewStoreRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: api_pb.NewStoreReply|null) => void
  ): UnaryResponse;
  newStore(
    requestMessage: api_pb.NewStoreRequest,
    callback: (error: ServiceError|null, responseMessage: api_pb.NewStoreReply|null) => void
  ): UnaryResponse;
  registerSchema(
    requestMessage: api_pb.RegisterSchemaRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: api_pb.RegisterSchemaReply|null) => void
  ): UnaryResponse;
  registerSchema(
    requestMessage: api_pb.RegisterSchemaRequest,
    callback: (error: ServiceError|null, responseMessage: api_pb.RegisterSchemaReply|null) => void
  ): UnaryResponse;
  start(
    requestMessage: api_pb.StartRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: api_pb.StartReply|null) => void
  ): UnaryResponse;
  start(
    requestMessage: api_pb.StartRequest,
    callback: (error: ServiceError|null, responseMessage: api_pb.StartReply|null) => void
  ): UnaryResponse;
  startFromAddress(
    requestMessage: api_pb.StartFromAddressRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: api_pb.StartFromAddressReply|null) => void
  ): UnaryResponse;
  startFromAddress(
    requestMessage: api_pb.StartFromAddressRequest,
    callback: (error: ServiceError|null, responseMessage: api_pb.StartFromAddressReply|null) => void
  ): UnaryResponse;
  modelCreate(
    requestMessage: api_pb.ModelCreateRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: api_pb.ModelCreateReply|null) => void
  ): UnaryResponse;
  modelCreate(
    requestMessage: api_pb.ModelCreateRequest,
    callback: (error: ServiceError|null, responseMessage: api_pb.ModelCreateReply|null) => void
  ): UnaryResponse;
  modelSave(
    requestMessage: api_pb.ModelSaveRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: api_pb.ModelSaveReply|null) => void
  ): UnaryResponse;
  modelSave(
    requestMessage: api_pb.ModelSaveRequest,
    callback: (error: ServiceError|null, responseMessage: api_pb.ModelSaveReply|null) => void
  ): UnaryResponse;
  modelDelete(
    requestMessage: api_pb.ModelDeleteRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: api_pb.ModelDeleteReply|null) => void
  ): UnaryResponse;
  modelDelete(
    requestMessage: api_pb.ModelDeleteRequest,
    callback: (error: ServiceError|null, responseMessage: api_pb.ModelDeleteReply|null) => void
  ): UnaryResponse;
  modelHas(
    requestMessage: api_pb.ModelHasRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: api_pb.ModelHasReply|null) => void
  ): UnaryResponse;
  modelHas(
    requestMessage: api_pb.ModelHasRequest,
    callback: (error: ServiceError|null, responseMessage: api_pb.ModelHasReply|null) => void
  ): UnaryResponse;
  modelFind(
    requestMessage: api_pb.ModelFindRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: api_pb.ModelFindReply|null) => void
  ): UnaryResponse;
  modelFind(
    requestMessage: api_pb.ModelFindRequest,
    callback: (error: ServiceError|null, responseMessage: api_pb.ModelFindReply|null) => void
  ): UnaryResponse;
  modelFindByID(
    requestMessage: api_pb.ModelFindByIDRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: api_pb.ModelFindByIDReply|null) => void
  ): UnaryResponse;
  modelFindByID(
    requestMessage: api_pb.ModelFindByIDRequest,
    callback: (error: ServiceError|null, responseMessage: api_pb.ModelFindByIDReply|null) => void
  ): UnaryResponse;
  readTransaction(metadata?: grpc.Metadata): BidirectionalStream<api_pb.ReadTransactionRequest, api_pb.ReadTransactionReply>;
  writeTransaction(metadata?: grpc.Metadata): BidirectionalStream<api_pb.WriteTransactionRequest, api_pb.WriteTransactionReply>;
  listen(requestMessage: api_pb.ListenRequest, metadata?: grpc.Metadata): ResponseStream<api_pb.ListenReply>;
}

