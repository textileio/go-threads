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

type APIModelCreate = {
  readonly methodName: string;
  readonly service: typeof API;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof api_pb.ModelCreateRequest;
  readonly responseType: typeof api_pb.ModelCreateReply;
};

export class API {
  static readonly serviceName: string;
  static readonly NewStore: APINewStore;
  static readonly RegisterSchema: APIRegisterSchema;
  static readonly ModelCreate: APIModelCreate;
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
  modelCreate(
    requestMessage: api_pb.ModelCreateRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: api_pb.ModelCreateReply|null) => void
  ): UnaryResponse;
  modelCreate(
    requestMessage: api_pb.ModelCreateRequest,
    callback: (error: ServiceError|null, responseMessage: api_pb.ModelCreateReply|null) => void
  ): UnaryResponse;
}

