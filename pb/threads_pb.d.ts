// package: threads.pb
// file: threads.proto

import * as jspb from "google-protobuf";
import * as github_com_gogo_protobuf_gogoproto_gogo_pb from "./github.com/gogo/protobuf/gogoproto/gogo_pb";

export class Log extends jspb.Message {
  getId(): Uint8Array | string;
  getId_asU8(): Uint8Array;
  getId_asB64(): string;
  setId(value: Uint8Array | string): void;

  getPubkey(): Uint8Array | string;
  getPubkey_asU8(): Uint8Array;
  getPubkey_asB64(): string;
  setPubkey(value: Uint8Array | string): void;

  clearAddrsList(): void;
  getAddrsList(): Array<Uint8Array | string>;
  getAddrsList_asU8(): Array<Uint8Array>;
  getAddrsList_asB64(): Array<string>;
  setAddrsList(value: Array<Uint8Array | string>): void;
  addAddrs(value: Uint8Array | string, index?: number): Uint8Array | string;

  clearHeadsList(): void;
  getHeadsList(): Array<Uint8Array | string>;
  getHeadsList_asU8(): Array<Uint8Array>;
  getHeadsList_asB64(): Array<string>;
  setHeadsList(value: Array<Uint8Array | string>): void;
  addHeads(value: Uint8Array | string, index?: number): Uint8Array | string;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): Log.AsObject;
  static toObject(includeInstance: boolean, msg: Log): Log.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: Log, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): Log;
  static deserializeBinaryFromReader(message: Log, reader: jspb.BinaryReader): Log;
}

export namespace Log {
  export type AsObject = {
    id: Uint8Array | string,
    pubkey: Uint8Array | string,
    addrsList: Array<Uint8Array | string>,
    headsList: Array<Uint8Array | string>,
  }

  export class Record extends jspb.Message {
    getRecordnode(): Uint8Array | string;
    getRecordnode_asU8(): Uint8Array;
    getRecordnode_asB64(): string;
    setRecordnode(value: Uint8Array | string): void;

    getEventnode(): Uint8Array | string;
    getEventnode_asU8(): Uint8Array;
    getEventnode_asB64(): string;
    setEventnode(value: Uint8Array | string): void;

    getHeadernode(): Uint8Array | string;
    getHeadernode_asU8(): Uint8Array;
    getHeadernode_asB64(): string;
    setHeadernode(value: Uint8Array | string): void;

    getBodynode(): Uint8Array | string;
    getBodynode_asU8(): Uint8Array;
    getBodynode_asB64(): string;
    setBodynode(value: Uint8Array | string): void;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Record.AsObject;
    static toObject(includeInstance: boolean, msg: Record): Record.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Record, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Record;
    static deserializeBinaryFromReader(message: Record, reader: jspb.BinaryReader): Record;
  }

  export namespace Record {
    export type AsObject = {
      recordnode: Uint8Array | string,
      eventnode: Uint8Array | string,
      headernode: Uint8Array | string,
      bodynode: Uint8Array | string,
    }
  }
}

export class GetLogsRequest extends jspb.Message {
  hasHeader(): boolean;
  clearHeader(): void;
  getHeader(): GetLogsRequest.Header | undefined;
  setHeader(value?: GetLogsRequest.Header): void;

  getThreadid(): Uint8Array | string;
  getThreadid_asU8(): Uint8Array;
  getThreadid_asB64(): string;
  setThreadid(value: Uint8Array | string): void;

  getFollowkey(): Uint8Array | string;
  getFollowkey_asU8(): Uint8Array;
  getFollowkey_asB64(): string;
  setFollowkey(value: Uint8Array | string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetLogsRequest.AsObject;
  static toObject(includeInstance: boolean, msg: GetLogsRequest): GetLogsRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetLogsRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetLogsRequest;
  static deserializeBinaryFromReader(message: GetLogsRequest, reader: jspb.BinaryReader): GetLogsRequest;
}

export namespace GetLogsRequest {
  export type AsObject = {
    header?: GetLogsRequest.Header.AsObject,
    threadid: Uint8Array | string,
    followkey: Uint8Array | string,
  }

  export class Header extends jspb.Message {
    getFrom(): Uint8Array | string;
    getFrom_asU8(): Uint8Array;
    getFrom_asB64(): string;
    setFrom(value: Uint8Array | string): void;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Header.AsObject;
    static toObject(includeInstance: boolean, msg: Header): Header.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Header, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Header;
    static deserializeBinaryFromReader(message: Header, reader: jspb.BinaryReader): Header;
  }

  export namespace Header {
    export type AsObject = {
      from: Uint8Array | string,
    }
  }
}

export class GetLogsReply extends jspb.Message {
  clearLogsList(): void;
  getLogsList(): Array<Log>;
  setLogsList(value: Array<Log>): void;
  addLogs(value?: Log, index?: number): Log;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetLogsReply.AsObject;
  static toObject(includeInstance: boolean, msg: GetLogsReply): GetLogsReply.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetLogsReply, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetLogsReply;
  static deserializeBinaryFromReader(message: GetLogsReply, reader: jspb.BinaryReader): GetLogsReply;
}

export namespace GetLogsReply {
  export type AsObject = {
    logsList: Array<Log.AsObject>,
  }
}

export class PushLogRequest extends jspb.Message {
  hasHeader(): boolean;
  clearHeader(): void;
  getHeader(): PushLogRequest.Header | undefined;
  setHeader(value?: PushLogRequest.Header): void;

  getThreadid(): Uint8Array | string;
  getThreadid_asU8(): Uint8Array;
  getThreadid_asB64(): string;
  setThreadid(value: Uint8Array | string): void;

  getFollowkey(): Uint8Array | string;
  getFollowkey_asU8(): Uint8Array;
  getFollowkey_asB64(): string;
  setFollowkey(value: Uint8Array | string): void;

  getReadkey(): Uint8Array | string;
  getReadkey_asU8(): Uint8Array;
  getReadkey_asB64(): string;
  setReadkey(value: Uint8Array | string): void;

  hasLog(): boolean;
  clearLog(): void;
  getLog(): Log | undefined;
  setLog(value?: Log): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): PushLogRequest.AsObject;
  static toObject(includeInstance: boolean, msg: PushLogRequest): PushLogRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: PushLogRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): PushLogRequest;
  static deserializeBinaryFromReader(message: PushLogRequest, reader: jspb.BinaryReader): PushLogRequest;
}

export namespace PushLogRequest {
  export type AsObject = {
    header?: PushLogRequest.Header.AsObject,
    threadid: Uint8Array | string,
    followkey: Uint8Array | string,
    readkey: Uint8Array | string,
    log?: Log.AsObject,
  }

  export class Header extends jspb.Message {
    getFrom(): Uint8Array | string;
    getFrom_asU8(): Uint8Array;
    getFrom_asB64(): string;
    setFrom(value: Uint8Array | string): void;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Header.AsObject;
    static toObject(includeInstance: boolean, msg: Header): Header.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Header, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Header;
    static deserializeBinaryFromReader(message: Header, reader: jspb.BinaryReader): Header;
  }

  export namespace Header {
    export type AsObject = {
      from: Uint8Array | string,
    }
  }
}

export class PushLogReply extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): PushLogReply.AsObject;
  static toObject(includeInstance: boolean, msg: PushLogReply): PushLogReply.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: PushLogReply, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): PushLogReply;
  static deserializeBinaryFromReader(message: PushLogReply, reader: jspb.BinaryReader): PushLogReply;
}

export namespace PushLogReply {
  export type AsObject = {
  }
}

export class GetRecordsRequest extends jspb.Message {
  hasHeader(): boolean;
  clearHeader(): void;
  getHeader(): GetRecordsRequest.Header | undefined;
  setHeader(value?: GetRecordsRequest.Header): void;

  getThreadid(): Uint8Array | string;
  getThreadid_asU8(): Uint8Array;
  getThreadid_asB64(): string;
  setThreadid(value: Uint8Array | string): void;

  getFollowkey(): Uint8Array | string;
  getFollowkey_asU8(): Uint8Array;
  getFollowkey_asB64(): string;
  setFollowkey(value: Uint8Array | string): void;

  clearLogsList(): void;
  getLogsList(): Array<GetRecordsRequest.LogEntry>;
  setLogsList(value: Array<GetRecordsRequest.LogEntry>): void;
  addLogs(value?: GetRecordsRequest.LogEntry, index?: number): GetRecordsRequest.LogEntry;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetRecordsRequest.AsObject;
  static toObject(includeInstance: boolean, msg: GetRecordsRequest): GetRecordsRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetRecordsRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetRecordsRequest;
  static deserializeBinaryFromReader(message: GetRecordsRequest, reader: jspb.BinaryReader): GetRecordsRequest;
}

export namespace GetRecordsRequest {
  export type AsObject = {
    header?: GetRecordsRequest.Header.AsObject,
    threadid: Uint8Array | string,
    followkey: Uint8Array | string,
    logsList: Array<GetRecordsRequest.LogEntry.AsObject>,
  }

  export class LogEntry extends jspb.Message {
    getLogid(): Uint8Array | string;
    getLogid_asU8(): Uint8Array;
    getLogid_asB64(): string;
    setLogid(value: Uint8Array | string): void;

    getOffset(): Uint8Array | string;
    getOffset_asU8(): Uint8Array;
    getOffset_asB64(): string;
    setOffset(value: Uint8Array | string): void;

    getLimit(): number;
    setLimit(value: number): void;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): LogEntry.AsObject;
    static toObject(includeInstance: boolean, msg: LogEntry): LogEntry.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: LogEntry, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): LogEntry;
    static deserializeBinaryFromReader(message: LogEntry, reader: jspb.BinaryReader): LogEntry;
  }

  export namespace LogEntry {
    export type AsObject = {
      logid: Uint8Array | string,
      offset: Uint8Array | string,
      limit: number,
    }
  }

  export class Header extends jspb.Message {
    getFrom(): Uint8Array | string;
    getFrom_asU8(): Uint8Array;
    getFrom_asB64(): string;
    setFrom(value: Uint8Array | string): void;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Header.AsObject;
    static toObject(includeInstance: boolean, msg: Header): Header.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Header, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Header;
    static deserializeBinaryFromReader(message: Header, reader: jspb.BinaryReader): Header;
  }

  export namespace Header {
    export type AsObject = {
      from: Uint8Array | string,
    }
  }
}

export class GetRecordsReply extends jspb.Message {
  clearLogsList(): void;
  getLogsList(): Array<GetRecordsReply.LogEntry>;
  setLogsList(value: Array<GetRecordsReply.LogEntry>): void;
  addLogs(value?: GetRecordsReply.LogEntry, index?: number): GetRecordsReply.LogEntry;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetRecordsReply.AsObject;
  static toObject(includeInstance: boolean, msg: GetRecordsReply): GetRecordsReply.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetRecordsReply, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetRecordsReply;
  static deserializeBinaryFromReader(message: GetRecordsReply, reader: jspb.BinaryReader): GetRecordsReply;
}

export namespace GetRecordsReply {
  export type AsObject = {
    logsList: Array<GetRecordsReply.LogEntry.AsObject>,
  }

  export class LogEntry extends jspb.Message {
    getLogid(): Uint8Array | string;
    getLogid_asU8(): Uint8Array;
    getLogid_asB64(): string;
    setLogid(value: Uint8Array | string): void;

    clearRecordsList(): void;
    getRecordsList(): Array<Log.Record>;
    setRecordsList(value: Array<Log.Record>): void;
    addRecords(value?: Log.Record, index?: number): Log.Record;

    hasLog(): boolean;
    clearLog(): void;
    getLog(): Log | undefined;
    setLog(value?: Log): void;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): LogEntry.AsObject;
    static toObject(includeInstance: boolean, msg: LogEntry): LogEntry.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: LogEntry, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): LogEntry;
    static deserializeBinaryFromReader(message: LogEntry, reader: jspb.BinaryReader): LogEntry;
  }

  export namespace LogEntry {
    export type AsObject = {
      logid: Uint8Array | string,
      recordsList: Array<Log.Record.AsObject>,
      log?: Log.AsObject,
    }
  }
}

export class PushRecordRequest extends jspb.Message {
  hasHeader(): boolean;
  clearHeader(): void;
  getHeader(): PushRecordRequest.Header | undefined;
  setHeader(value?: PushRecordRequest.Header): void;

  getThreadid(): Uint8Array | string;
  getThreadid_asU8(): Uint8Array;
  getThreadid_asB64(): string;
  setThreadid(value: Uint8Array | string): void;

  getLogid(): Uint8Array | string;
  getLogid_asU8(): Uint8Array;
  getLogid_asB64(): string;
  setLogid(value: Uint8Array | string): void;

  hasRecord(): boolean;
  clearRecord(): void;
  getRecord(): Log.Record | undefined;
  setRecord(value?: Log.Record): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): PushRecordRequest.AsObject;
  static toObject(includeInstance: boolean, msg: PushRecordRequest): PushRecordRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: PushRecordRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): PushRecordRequest;
  static deserializeBinaryFromReader(message: PushRecordRequest, reader: jspb.BinaryReader): PushRecordRequest;
}

export namespace PushRecordRequest {
  export type AsObject = {
    header?: PushRecordRequest.Header.AsObject,
    threadid: Uint8Array | string,
    logid: Uint8Array | string,
    record?: Log.Record.AsObject,
  }

  export class Header extends jspb.Message {
    getFrom(): Uint8Array | string;
    getFrom_asU8(): Uint8Array;
    getFrom_asB64(): string;
    setFrom(value: Uint8Array | string): void;

    getSignature(): Uint8Array | string;
    getSignature_asU8(): Uint8Array;
    getSignature_asB64(): string;
    setSignature(value: Uint8Array | string): void;

    getKey(): Uint8Array | string;
    getKey_asU8(): Uint8Array;
    getKey_asB64(): string;
    setKey(value: Uint8Array | string): void;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Header.AsObject;
    static toObject(includeInstance: boolean, msg: Header): Header.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Header, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Header;
    static deserializeBinaryFromReader(message: Header, reader: jspb.BinaryReader): Header;
  }

  export namespace Header {
    export type AsObject = {
      from: Uint8Array | string,
      signature: Uint8Array | string,
      key: Uint8Array | string,
    }
  }
}

export class PushRecordReply extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): PushRecordReply.AsObject;
  static toObject(includeInstance: boolean, msg: PushRecordReply): PushRecordReply.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: PushRecordReply, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): PushRecordReply;
  static deserializeBinaryFromReader(message: PushRecordReply, reader: jspb.BinaryReader): PushRecordReply;
}

export namespace PushRecordReply {
  export type AsObject = {
  }
}

