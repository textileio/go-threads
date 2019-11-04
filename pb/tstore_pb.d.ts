// package: threads.pb
// file: tstore.proto

import * as jspb from "google-protobuf";
import * as github_com_gogo_protobuf_gogoproto_gogo_pb from "./github.com/gogo/protobuf/gogoproto/gogo_pb";

export class AddrBookRecord extends jspb.Message {
  getThreadid(): Uint8Array | string;
  getThreadid_asU8(): Uint8Array;
  getThreadid_asB64(): string;
  setThreadid(value: Uint8Array | string): void;

  getPeerid(): Uint8Array | string;
  getPeerid_asU8(): Uint8Array;
  getPeerid_asB64(): string;
  setPeerid(value: Uint8Array | string): void;

  clearAddrsList(): void;
  getAddrsList(): Array<AddrBookRecord.AddrEntry>;
  setAddrsList(value: Array<AddrBookRecord.AddrEntry>): void;
  addAddrs(value?: AddrBookRecord.AddrEntry, index?: number): AddrBookRecord.AddrEntry;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): AddrBookRecord.AsObject;
  static toObject(includeInstance: boolean, msg: AddrBookRecord): AddrBookRecord.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: AddrBookRecord, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): AddrBookRecord;
  static deserializeBinaryFromReader(message: AddrBookRecord, reader: jspb.BinaryReader): AddrBookRecord;
}

export namespace AddrBookRecord {
  export type AsObject = {
    threadid: Uint8Array | string,
    peerid: Uint8Array | string,
    addrsList: Array<AddrBookRecord.AddrEntry.AsObject>,
  }

  export class AddrEntry extends jspb.Message {
    getAddr(): Uint8Array | string;
    getAddr_asU8(): Uint8Array;
    getAddr_asB64(): string;
    setAddr(value: Uint8Array | string): void;

    getExpiry(): number;
    setExpiry(value: number): void;

    getTtl(): number;
    setTtl(value: number): void;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): AddrEntry.AsObject;
    static toObject(includeInstance: boolean, msg: AddrEntry): AddrEntry.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: AddrEntry, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): AddrEntry;
    static deserializeBinaryFromReader(message: AddrEntry, reader: jspb.BinaryReader): AddrEntry;
  }

  export namespace AddrEntry {
    export type AsObject = {
      addr: Uint8Array | string,
      expiry: number,
      ttl: number,
    }
  }
}

export class HeadBookRecord extends jspb.Message {
  clearHeadsList(): void;
  getHeadsList(): Array<HeadBookRecord.HeadEntry>;
  setHeadsList(value: Array<HeadBookRecord.HeadEntry>): void;
  addHeads(value?: HeadBookRecord.HeadEntry, index?: number): HeadBookRecord.HeadEntry;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): HeadBookRecord.AsObject;
  static toObject(includeInstance: boolean, msg: HeadBookRecord): HeadBookRecord.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: HeadBookRecord, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): HeadBookRecord;
  static deserializeBinaryFromReader(message: HeadBookRecord, reader: jspb.BinaryReader): HeadBookRecord;
}

export namespace HeadBookRecord {
  export type AsObject = {
    headsList: Array<HeadBookRecord.HeadEntry.AsObject>,
  }

  export class HeadEntry extends jspb.Message {
    getCid(): Uint8Array | string;
    getCid_asU8(): Uint8Array;
    getCid_asB64(): string;
    setCid(value: Uint8Array | string): void;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): HeadEntry.AsObject;
    static toObject(includeInstance: boolean, msg: HeadEntry): HeadEntry.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: HeadEntry, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): HeadEntry;
    static deserializeBinaryFromReader(message: HeadEntry, reader: jspb.BinaryReader): HeadEntry;
  }

  export namespace HeadEntry {
    export type AsObject = {
      cid: Uint8Array | string,
    }
  }
}

