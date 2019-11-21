// package: api.pb
// file: api.proto

import * as jspb from "google-protobuf";

export class NewStoreRequest extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): NewStoreRequest.AsObject;
  static toObject(includeInstance: boolean, msg: NewStoreRequest): NewStoreRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: NewStoreRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): NewStoreRequest;
  static deserializeBinaryFromReader(message: NewStoreRequest, reader: jspb.BinaryReader): NewStoreRequest;
}

export namespace NewStoreRequest {
  export type AsObject = {
  }
}

export class NewStoreReply extends jspb.Message {
  getId(): string;
  setId(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): NewStoreReply.AsObject;
  static toObject(includeInstance: boolean, msg: NewStoreReply): NewStoreReply.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: NewStoreReply, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): NewStoreReply;
  static deserializeBinaryFromReader(message: NewStoreReply, reader: jspb.BinaryReader): NewStoreReply;
}

export namespace NewStoreReply {
  export type AsObject = {
    id: string,
  }
}

export class RegisterSchemaRequest extends jspb.Message {
  getStoreid(): string;
  setStoreid(value: string): void;

  getName(): string;
  setName(value: string): void;

  getSchema(): string;
  setSchema(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): RegisterSchemaRequest.AsObject;
  static toObject(includeInstance: boolean, msg: RegisterSchemaRequest): RegisterSchemaRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: RegisterSchemaRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): RegisterSchemaRequest;
  static deserializeBinaryFromReader(message: RegisterSchemaRequest, reader: jspb.BinaryReader): RegisterSchemaRequest;
}

export namespace RegisterSchemaRequest {
  export type AsObject = {
    storeid: string,
    name: string,
    schema: string,
  }
}

export class RegisterSchemaReply extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): RegisterSchemaReply.AsObject;
  static toObject(includeInstance: boolean, msg: RegisterSchemaReply): RegisterSchemaReply.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: RegisterSchemaReply, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): RegisterSchemaReply;
  static deserializeBinaryFromReader(message: RegisterSchemaReply, reader: jspb.BinaryReader): RegisterSchemaReply;
}

export namespace RegisterSchemaReply {
  export type AsObject = {
  }
}

export class ModelCreateRequest extends jspb.Message {
  getStoreid(): string;
  setStoreid(value: string): void;

  getModelname(): string;
  setModelname(value: string): void;

  clearValuesList(): void;
  getValuesList(): Array<string>;
  setValuesList(value: Array<string>): void;
  addValues(value: string, index?: number): string;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ModelCreateRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ModelCreateRequest): ModelCreateRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ModelCreateRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ModelCreateRequest;
  static deserializeBinaryFromReader(message: ModelCreateRequest, reader: jspb.BinaryReader): ModelCreateRequest;
}

export namespace ModelCreateRequest {
  export type AsObject = {
    storeid: string,
    modelname: string,
    valuesList: Array<string>,
  }
}

export class ModelCreateReply extends jspb.Message {
  clearEntitiesList(): void;
  getEntitiesList(): Array<string>;
  setEntitiesList(value: Array<string>): void;
  addEntities(value: string, index?: number): string;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ModelCreateReply.AsObject;
  static toObject(includeInstance: boolean, msg: ModelCreateReply): ModelCreateReply.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ModelCreateReply, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ModelCreateReply;
  static deserializeBinaryFromReader(message: ModelCreateReply, reader: jspb.BinaryReader): ModelCreateReply;
}

export namespace ModelCreateReply {
  export type AsObject = {
    entitiesList: Array<string>,
  }
}

