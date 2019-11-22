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

export class ModelSaveRequest extends jspb.Message {
  getStoreid(): string;
  setStoreid(value: string): void;

  getModelname(): string;
  setModelname(value: string): void;

  clearValuesList(): void;
  getValuesList(): Array<string>;
  setValuesList(value: Array<string>): void;
  addValues(value: string, index?: number): string;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ModelSaveRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ModelSaveRequest): ModelSaveRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ModelSaveRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ModelSaveRequest;
  static deserializeBinaryFromReader(message: ModelSaveRequest, reader: jspb.BinaryReader): ModelSaveRequest;
}

export namespace ModelSaveRequest {
  export type AsObject = {
    storeid: string,
    modelname: string,
    valuesList: Array<string>,
  }
}

export class ModelSaveReply extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ModelSaveReply.AsObject;
  static toObject(includeInstance: boolean, msg: ModelSaveReply): ModelSaveReply.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ModelSaveReply, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ModelSaveReply;
  static deserializeBinaryFromReader(message: ModelSaveReply, reader: jspb.BinaryReader): ModelSaveReply;
}

export namespace ModelSaveReply {
  export type AsObject = {
  }
}

export class ModelDeleteRequest extends jspb.Message {
  getStoreid(): string;
  setStoreid(value: string): void;

  getModelname(): string;
  setModelname(value: string): void;

  getEntityid(): string;
  setEntityid(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ModelDeleteRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ModelDeleteRequest): ModelDeleteRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ModelDeleteRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ModelDeleteRequest;
  static deserializeBinaryFromReader(message: ModelDeleteRequest, reader: jspb.BinaryReader): ModelDeleteRequest;
}

export namespace ModelDeleteRequest {
  export type AsObject = {
    storeid: string,
    modelname: string,
    entityid: string,
  }
}

export class ModelDeleteReply extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ModelDeleteReply.AsObject;
  static toObject(includeInstance: boolean, msg: ModelDeleteReply): ModelDeleteReply.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ModelDeleteReply, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ModelDeleteReply;
  static deserializeBinaryFromReader(message: ModelDeleteReply, reader: jspb.BinaryReader): ModelDeleteReply;
}

export namespace ModelDeleteReply {
  export type AsObject = {
  }
}

export class ModelHasRequest extends jspb.Message {
  getStoreid(): string;
  setStoreid(value: string): void;

  getModelname(): string;
  setModelname(value: string): void;

  getEntityid(): string;
  setEntityid(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ModelHasRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ModelHasRequest): ModelHasRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ModelHasRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ModelHasRequest;
  static deserializeBinaryFromReader(message: ModelHasRequest, reader: jspb.BinaryReader): ModelHasRequest;
}

export namespace ModelHasRequest {
  export type AsObject = {
    storeid: string,
    modelname: string,
    entityid: string,
  }
}

export class ModelHasReply extends jspb.Message {
  getExists(): boolean;
  setExists(value: boolean): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ModelHasReply.AsObject;
  static toObject(includeInstance: boolean, msg: ModelHasReply): ModelHasReply.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ModelHasReply, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ModelHasReply;
  static deserializeBinaryFromReader(message: ModelHasReply, reader: jspb.BinaryReader): ModelHasReply;
}

export namespace ModelHasReply {
  export type AsObject = {
    exists: boolean,
  }
}

export class ModelFindRequest extends jspb.Message {
  getStoreid(): string;
  setStoreid(value: string): void;

  getModelname(): string;
  setModelname(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ModelFindRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ModelFindRequest): ModelFindRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ModelFindRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ModelFindRequest;
  static deserializeBinaryFromReader(message: ModelFindRequest, reader: jspb.BinaryReader): ModelFindRequest;
}

export namespace ModelFindRequest {
  export type AsObject = {
    storeid: string,
    modelname: string,
  }
}

export class ModelFindReply extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ModelFindReply.AsObject;
  static toObject(includeInstance: boolean, msg: ModelFindReply): ModelFindReply.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ModelFindReply, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ModelFindReply;
  static deserializeBinaryFromReader(message: ModelFindReply, reader: jspb.BinaryReader): ModelFindReply;
}

export namespace ModelFindReply {
  export type AsObject = {
  }
}

export class ModelFindByIDRequest extends jspb.Message {
  getStoreid(): string;
  setStoreid(value: string): void;

  getModelname(): string;
  setModelname(value: string): void;

  getEntityid(): string;
  setEntityid(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ModelFindByIDRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ModelFindByIDRequest): ModelFindByIDRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ModelFindByIDRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ModelFindByIDRequest;
  static deserializeBinaryFromReader(message: ModelFindByIDRequest, reader: jspb.BinaryReader): ModelFindByIDRequest;
}

export namespace ModelFindByIDRequest {
  export type AsObject = {
    storeid: string,
    modelname: string,
    entityid: string,
  }
}

export class ModelFindByIDReply extends jspb.Message {
  getEntity(): string;
  setEntity(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ModelFindByIDReply.AsObject;
  static toObject(includeInstance: boolean, msg: ModelFindByIDReply): ModelFindByIDReply.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ModelFindByIDReply, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ModelFindByIDReply;
  static deserializeBinaryFromReader(message: ModelFindByIDReply, reader: jspb.BinaryReader): ModelFindByIDReply;
}

export namespace ModelFindByIDReply {
  export type AsObject = {
    entity: string,
  }
}

export class StartTransactionRequest extends jspb.Message {
  getStoreid(): string;
  setStoreid(value: string): void;

  getModelname(): string;
  setModelname(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): StartTransactionRequest.AsObject;
  static toObject(includeInstance: boolean, msg: StartTransactionRequest): StartTransactionRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: StartTransactionRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): StartTransactionRequest;
  static deserializeBinaryFromReader(message: StartTransactionRequest, reader: jspb.BinaryReader): StartTransactionRequest;
}

export namespace StartTransactionRequest {
  export type AsObject = {
    storeid: string,
    modelname: string,
  }
}

export class ReadTransactionRequest extends jspb.Message {
  hasStarttransactionrequest(): boolean;
  clearStarttransactionrequest(): void;
  getStarttransactionrequest(): StartTransactionRequest | undefined;
  setStarttransactionrequest(value?: StartTransactionRequest): void;

  hasModelhasrequest(): boolean;
  clearModelhasrequest(): void;
  getModelhasrequest(): ModelHasRequest | undefined;
  setModelhasrequest(value?: ModelHasRequest): void;

  hasModelfindrequest(): boolean;
  clearModelfindrequest(): void;
  getModelfindrequest(): ModelFindRequest | undefined;
  setModelfindrequest(value?: ModelFindRequest): void;

  hasModelfindbyidrequest(): boolean;
  clearModelfindbyidrequest(): void;
  getModelfindbyidrequest(): ModelFindByIDRequest | undefined;
  setModelfindbyidrequest(value?: ModelFindByIDRequest): void;

  getOptionCase(): ReadTransactionRequest.OptionCase;
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ReadTransactionRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ReadTransactionRequest): ReadTransactionRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ReadTransactionRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ReadTransactionRequest;
  static deserializeBinaryFromReader(message: ReadTransactionRequest, reader: jspb.BinaryReader): ReadTransactionRequest;
}

export namespace ReadTransactionRequest {
  export type AsObject = {
    starttransactionrequest?: StartTransactionRequest.AsObject,
    modelhasrequest?: ModelHasRequest.AsObject,
    modelfindrequest?: ModelFindRequest.AsObject,
    modelfindbyidrequest?: ModelFindByIDRequest.AsObject,
  }

  export enum OptionCase {
    OPTION_NOT_SET = 0,
    STARTTRANSACTIONREQUEST = 1,
    MODELHASREQUEST = 2,
    MODELFINDREQUEST = 3,
    MODELFINDBYIDREQUEST = 4,
  }
}

export class ReadTransactionReply extends jspb.Message {
  hasModelhasreply(): boolean;
  clearModelhasreply(): void;
  getModelhasreply(): ModelHasReply | undefined;
  setModelhasreply(value?: ModelHasReply): void;

  hasModelfindreply(): boolean;
  clearModelfindreply(): void;
  getModelfindreply(): ModelFindReply | undefined;
  setModelfindreply(value?: ModelFindReply): void;

  hasModelfindbyidreply(): boolean;
  clearModelfindbyidreply(): void;
  getModelfindbyidreply(): ModelFindByIDReply | undefined;
  setModelfindbyidreply(value?: ModelFindByIDReply): void;

  getOptionCase(): ReadTransactionReply.OptionCase;
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ReadTransactionReply.AsObject;
  static toObject(includeInstance: boolean, msg: ReadTransactionReply): ReadTransactionReply.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ReadTransactionReply, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ReadTransactionReply;
  static deserializeBinaryFromReader(message: ReadTransactionReply, reader: jspb.BinaryReader): ReadTransactionReply;
}

export namespace ReadTransactionReply {
  export type AsObject = {
    modelhasreply?: ModelHasReply.AsObject,
    modelfindreply?: ModelFindReply.AsObject,
    modelfindbyidreply?: ModelFindByIDReply.AsObject,
  }

  export enum OptionCase {
    OPTION_NOT_SET = 0,
    MODELHASREPLY = 1,
    MODELFINDREPLY = 2,
    MODELFINDBYIDREPLY = 3,
  }
}

export class WriteTransactionRequest extends jspb.Message {
  hasStarttransactionrequest(): boolean;
  clearStarttransactionrequest(): void;
  getStarttransactionrequest(): StartTransactionRequest | undefined;
  setStarttransactionrequest(value?: StartTransactionRequest): void;

  hasModelcreaterequest(): boolean;
  clearModelcreaterequest(): void;
  getModelcreaterequest(): ModelCreateRequest | undefined;
  setModelcreaterequest(value?: ModelCreateRequest): void;

  hasModelsaverequest(): boolean;
  clearModelsaverequest(): void;
  getModelsaverequest(): ModelSaveRequest | undefined;
  setModelsaverequest(value?: ModelSaveRequest): void;

  hasModeldeleterequest(): boolean;
  clearModeldeleterequest(): void;
  getModeldeleterequest(): ModelDeleteRequest | undefined;
  setModeldeleterequest(value?: ModelDeleteRequest): void;

  hasModelhasrequest(): boolean;
  clearModelhasrequest(): void;
  getModelhasrequest(): ModelHasRequest | undefined;
  setModelhasrequest(value?: ModelHasRequest): void;

  hasModelfindrequest(): boolean;
  clearModelfindrequest(): void;
  getModelfindrequest(): ModelFindRequest | undefined;
  setModelfindrequest(value?: ModelFindRequest): void;

  hasModelfindbyidrequest(): boolean;
  clearModelfindbyidrequest(): void;
  getModelfindbyidrequest(): ModelFindByIDRequest | undefined;
  setModelfindbyidrequest(value?: ModelFindByIDRequest): void;

  getOptionCase(): WriteTransactionRequest.OptionCase;
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): WriteTransactionRequest.AsObject;
  static toObject(includeInstance: boolean, msg: WriteTransactionRequest): WriteTransactionRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: WriteTransactionRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): WriteTransactionRequest;
  static deserializeBinaryFromReader(message: WriteTransactionRequest, reader: jspb.BinaryReader): WriteTransactionRequest;
}

export namespace WriteTransactionRequest {
  export type AsObject = {
    starttransactionrequest?: StartTransactionRequest.AsObject,
    modelcreaterequest?: ModelCreateRequest.AsObject,
    modelsaverequest?: ModelSaveRequest.AsObject,
    modeldeleterequest?: ModelDeleteRequest.AsObject,
    modelhasrequest?: ModelHasRequest.AsObject,
    modelfindrequest?: ModelFindRequest.AsObject,
    modelfindbyidrequest?: ModelFindByIDRequest.AsObject,
  }

  export enum OptionCase {
    OPTION_NOT_SET = 0,
    STARTTRANSACTIONREQUEST = 1,
    MODELCREATEREQUEST = 2,
    MODELSAVEREQUEST = 3,
    MODELDELETEREQUEST = 4,
    MODELHASREQUEST = 5,
    MODELFINDREQUEST = 6,
    MODELFINDBYIDREQUEST = 7,
  }
}

export class WriteTransactionReply extends jspb.Message {
  hasModelcreatereply(): boolean;
  clearModelcreatereply(): void;
  getModelcreatereply(): ModelCreateReply | undefined;
  setModelcreatereply(value?: ModelCreateReply): void;

  hasModelsavereply(): boolean;
  clearModelsavereply(): void;
  getModelsavereply(): ModelSaveReply | undefined;
  setModelsavereply(value?: ModelSaveReply): void;

  hasModeldeletereply(): boolean;
  clearModeldeletereply(): void;
  getModeldeletereply(): ModelDeleteReply | undefined;
  setModeldeletereply(value?: ModelDeleteReply): void;

  hasModelhasreply(): boolean;
  clearModelhasreply(): void;
  getModelhasreply(): ModelHasReply | undefined;
  setModelhasreply(value?: ModelHasReply): void;

  hasModelfindreply(): boolean;
  clearModelfindreply(): void;
  getModelfindreply(): ModelFindReply | undefined;
  setModelfindreply(value?: ModelFindReply): void;

  hasModelfindbyidreply(): boolean;
  clearModelfindbyidreply(): void;
  getModelfindbyidreply(): ModelFindByIDReply | undefined;
  setModelfindbyidreply(value?: ModelFindByIDReply): void;

  getOptionCase(): WriteTransactionReply.OptionCase;
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): WriteTransactionReply.AsObject;
  static toObject(includeInstance: boolean, msg: WriteTransactionReply): WriteTransactionReply.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: WriteTransactionReply, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): WriteTransactionReply;
  static deserializeBinaryFromReader(message: WriteTransactionReply, reader: jspb.BinaryReader): WriteTransactionReply;
}

export namespace WriteTransactionReply {
  export type AsObject = {
    modelcreatereply?: ModelCreateReply.AsObject,
    modelsavereply?: ModelSaveReply.AsObject,
    modeldeletereply?: ModelDeleteReply.AsObject,
    modelhasreply?: ModelHasReply.AsObject,
    modelfindreply?: ModelFindReply.AsObject,
    modelfindbyidreply?: ModelFindByIDReply.AsObject,
  }

  export enum OptionCase {
    OPTION_NOT_SET = 0,
    MODELCREATEREPLY = 1,
    MODELSAVEREPLY = 2,
    MODELDELETEREPLY = 3,
    MODELHASREPLY = 4,
    MODELFINDREPLY = 5,
    MODELFINDBYIDREPLY = 6,
  }
}

export class ListenRequest extends jspb.Message {
  getStoreid(): string;
  setStoreid(value: string): void;

  getModelname(): string;
  setModelname(value: string): void;

  getEntityid(): string;
  setEntityid(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ListenRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ListenRequest): ListenRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ListenRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ListenRequest;
  static deserializeBinaryFromReader(message: ListenRequest, reader: jspb.BinaryReader): ListenRequest;
}

export namespace ListenRequest {
  export type AsObject = {
    storeid: string,
    modelname: string,
    entityid: string,
  }
}

export class ListenReply extends jspb.Message {
  getEntity(): string;
  setEntity(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ListenReply.AsObject;
  static toObject(includeInstance: boolean, msg: ListenReply): ListenReply.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ListenReply, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ListenReply;
  static deserializeBinaryFromReader(message: ListenReply, reader: jspb.BinaryReader): ListenReply;
}

export namespace ListenReply {
  export type AsObject = {
    entity: string,
  }
}

