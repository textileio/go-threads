///
//  Generated code. Do not modify.
//  source: api.proto
//
// @dart = 2.3
// ignore_for_file: camel_case_types,non_constant_identifier_names,library_prefixes,unused_import,unused_shown_name,return_of_invalid_type

const NewStoreRequest$json = const {
  '1': 'NewStoreRequest',
};

const NewStoreReply$json = const {
  '1': 'NewStoreReply',
  '2': const [
    const {'1': 'ID', '3': 1, '4': 1, '5': 9, '10': 'ID'},
  ],
};

const RegisterSchemaRequest$json = const {
  '1': 'RegisterSchemaRequest',
  '2': const [
    const {'1': 'storeID', '3': 1, '4': 1, '5': 9, '10': 'storeID'},
    const {'1': 'name', '3': 2, '4': 1, '5': 9, '10': 'name'},
    const {'1': 'schema', '3': 3, '4': 1, '5': 9, '10': 'schema'},
  ],
};

const RegisterSchemaReply$json = const {
  '1': 'RegisterSchemaReply',
};

const StartRequest$json = const {
  '1': 'StartRequest',
  '2': const [
    const {'1': 'storeID', '3': 1, '4': 1, '5': 9, '10': 'storeID'},
  ],
};

const StartReply$json = const {
  '1': 'StartReply',
};

const StartFromAddressRequest$json = const {
  '1': 'StartFromAddressRequest',
  '2': const [
    const {'1': 'storeID', '3': 1, '4': 1, '5': 9, '10': 'storeID'},
    const {'1': 'address', '3': 2, '4': 1, '5': 9, '10': 'address'},
    const {'1': 'followKey', '3': 3, '4': 1, '5': 12, '10': 'followKey'},
    const {'1': 'readKey', '3': 4, '4': 1, '5': 12, '10': 'readKey'},
  ],
};

const StartFromAddressReply$json = const {
  '1': 'StartFromAddressReply',
};

const GetStoreLinkRequest$json = const {
  '1': 'GetStoreLinkRequest',
  '2': const [
    const {'1': 'storeID', '3': 1, '4': 1, '5': 9, '10': 'storeID'},
  ],
};

const GetStoreLinkReply$json = const {
  '1': 'GetStoreLinkReply',
  '2': const [
    const {'1': 'addresses', '3': 1, '4': 3, '5': 9, '10': 'addresses'},
    const {'1': 'followKey', '3': 2, '4': 1, '5': 12, '10': 'followKey'},
    const {'1': 'readKey', '3': 3, '4': 1, '5': 12, '10': 'readKey'},
  ],
};

const ModelCreateRequest$json = const {
  '1': 'ModelCreateRequest',
  '2': const [
    const {'1': 'storeID', '3': 1, '4': 1, '5': 9, '10': 'storeID'},
    const {'1': 'modelName', '3': 2, '4': 1, '5': 9, '10': 'modelName'},
    const {'1': 'values', '3': 3, '4': 3, '5': 9, '10': 'values'},
  ],
};

const ModelCreateReply$json = const {
  '1': 'ModelCreateReply',
  '2': const [
    const {'1': 'entities', '3': 1, '4': 3, '5': 9, '10': 'entities'},
  ],
};

const ModelSaveRequest$json = const {
  '1': 'ModelSaveRequest',
  '2': const [
    const {'1': 'storeID', '3': 1, '4': 1, '5': 9, '10': 'storeID'},
    const {'1': 'modelName', '3': 2, '4': 1, '5': 9, '10': 'modelName'},
    const {'1': 'values', '3': 3, '4': 3, '5': 9, '10': 'values'},
  ],
};

const ModelSaveReply$json = const {
  '1': 'ModelSaveReply',
};

const ModelDeleteRequest$json = const {
  '1': 'ModelDeleteRequest',
  '2': const [
    const {'1': 'storeID', '3': 1, '4': 1, '5': 9, '10': 'storeID'},
    const {'1': 'modelName', '3': 2, '4': 1, '5': 9, '10': 'modelName'},
    const {'1': 'entityIDs', '3': 3, '4': 3, '5': 9, '10': 'entityIDs'},
  ],
};

const ModelDeleteReply$json = const {
  '1': 'ModelDeleteReply',
};

const ModelHasRequest$json = const {
  '1': 'ModelHasRequest',
  '2': const [
    const {'1': 'storeID', '3': 1, '4': 1, '5': 9, '10': 'storeID'},
    const {'1': 'modelName', '3': 2, '4': 1, '5': 9, '10': 'modelName'},
    const {'1': 'entityIDs', '3': 3, '4': 3, '5': 9, '10': 'entityIDs'},
  ],
};

const ModelHasReply$json = const {
  '1': 'ModelHasReply',
  '2': const [
    const {'1': 'exists', '3': 1, '4': 1, '5': 8, '10': 'exists'},
  ],
};

const ModelFindRequest$json = const {
  '1': 'ModelFindRequest',
  '2': const [
    const {'1': 'storeID', '3': 1, '4': 1, '5': 9, '10': 'storeID'},
    const {'1': 'modelName', '3': 2, '4': 1, '5': 9, '10': 'modelName'},
    const {'1': 'queryJSON', '3': 3, '4': 1, '5': 12, '10': 'queryJSON'},
  ],
};

const ModelFindReply$json = const {
  '1': 'ModelFindReply',
  '2': const [
    const {'1': 'entities', '3': 1, '4': 3, '5': 12, '10': 'entities'},
  ],
};

const ModelFindByIDRequest$json = const {
  '1': 'ModelFindByIDRequest',
  '2': const [
    const {'1': 'storeID', '3': 1, '4': 1, '5': 9, '10': 'storeID'},
    const {'1': 'modelName', '3': 2, '4': 1, '5': 9, '10': 'modelName'},
    const {'1': 'entityID', '3': 3, '4': 1, '5': 9, '10': 'entityID'},
  ],
};

const ModelFindByIDReply$json = const {
  '1': 'ModelFindByIDReply',
  '2': const [
    const {'1': 'entity', '3': 1, '4': 1, '5': 9, '10': 'entity'},
  ],
};

const StartTransactionRequest$json = const {
  '1': 'StartTransactionRequest',
  '2': const [
    const {'1': 'storeID', '3': 1, '4': 1, '5': 9, '10': 'storeID'},
    const {'1': 'modelName', '3': 2, '4': 1, '5': 9, '10': 'modelName'},
  ],
};

const ReadTransactionRequest$json = const {
  '1': 'ReadTransactionRequest',
  '2': const [
    const {'1': 'startTransactionRequest', '3': 1, '4': 1, '5': 11, '6': '.api.pb.StartTransactionRequest', '9': 0, '10': 'startTransactionRequest'},
    const {'1': 'modelHasRequest', '3': 2, '4': 1, '5': 11, '6': '.api.pb.ModelHasRequest', '9': 0, '10': 'modelHasRequest'},
    const {'1': 'modelFindRequest', '3': 3, '4': 1, '5': 11, '6': '.api.pb.ModelFindRequest', '9': 0, '10': 'modelFindRequest'},
    const {'1': 'modelFindByIDRequest', '3': 4, '4': 1, '5': 11, '6': '.api.pb.ModelFindByIDRequest', '9': 0, '10': 'modelFindByIDRequest'},
  ],
  '8': const [
    const {'1': 'option'},
  ],
};

const ReadTransactionReply$json = const {
  '1': 'ReadTransactionReply',
  '2': const [
    const {'1': 'modelHasReply', '3': 1, '4': 1, '5': 11, '6': '.api.pb.ModelHasReply', '9': 0, '10': 'modelHasReply'},
    const {'1': 'modelFindReply', '3': 2, '4': 1, '5': 11, '6': '.api.pb.ModelFindReply', '9': 0, '10': 'modelFindReply'},
    const {'1': 'modelFindByIDReply', '3': 3, '4': 1, '5': 11, '6': '.api.pb.ModelFindByIDReply', '9': 0, '10': 'modelFindByIDReply'},
  ],
  '8': const [
    const {'1': 'option'},
  ],
};

const WriteTransactionRequest$json = const {
  '1': 'WriteTransactionRequest',
  '2': const [
    const {'1': 'startTransactionRequest', '3': 1, '4': 1, '5': 11, '6': '.api.pb.StartTransactionRequest', '9': 0, '10': 'startTransactionRequest'},
    const {'1': 'modelCreateRequest', '3': 2, '4': 1, '5': 11, '6': '.api.pb.ModelCreateRequest', '9': 0, '10': 'modelCreateRequest'},
    const {'1': 'modelSaveRequest', '3': 3, '4': 1, '5': 11, '6': '.api.pb.ModelSaveRequest', '9': 0, '10': 'modelSaveRequest'},
    const {'1': 'modelDeleteRequest', '3': 4, '4': 1, '5': 11, '6': '.api.pb.ModelDeleteRequest', '9': 0, '10': 'modelDeleteRequest'},
    const {'1': 'modelHasRequest', '3': 5, '4': 1, '5': 11, '6': '.api.pb.ModelHasRequest', '9': 0, '10': 'modelHasRequest'},
    const {'1': 'modelFindRequest', '3': 6, '4': 1, '5': 11, '6': '.api.pb.ModelFindRequest', '9': 0, '10': 'modelFindRequest'},
    const {'1': 'modelFindByIDRequest', '3': 7, '4': 1, '5': 11, '6': '.api.pb.ModelFindByIDRequest', '9': 0, '10': 'modelFindByIDRequest'},
  ],
  '8': const [
    const {'1': 'option'},
  ],
};

const WriteTransactionReply$json = const {
  '1': 'WriteTransactionReply',
  '2': const [
    const {'1': 'modelCreateReply', '3': 1, '4': 1, '5': 11, '6': '.api.pb.ModelCreateReply', '9': 0, '10': 'modelCreateReply'},
    const {'1': 'modelSaveReply', '3': 2, '4': 1, '5': 11, '6': '.api.pb.ModelSaveReply', '9': 0, '10': 'modelSaveReply'},
    const {'1': 'modelDeleteReply', '3': 3, '4': 1, '5': 11, '6': '.api.pb.ModelDeleteReply', '9': 0, '10': 'modelDeleteReply'},
    const {'1': 'modelHasReply', '3': 4, '4': 1, '5': 11, '6': '.api.pb.ModelHasReply', '9': 0, '10': 'modelHasReply'},
    const {'1': 'modelFindReply', '3': 5, '4': 1, '5': 11, '6': '.api.pb.ModelFindReply', '9': 0, '10': 'modelFindReply'},
    const {'1': 'modelFindByIDReply', '3': 6, '4': 1, '5': 11, '6': '.api.pb.ModelFindByIDReply', '9': 0, '10': 'modelFindByIDReply'},
  ],
  '8': const [
    const {'1': 'option'},
  ],
};

const ListenRequest$json = const {
  '1': 'ListenRequest',
  '2': const [
    const {'1': 'storeID', '3': 1, '4': 1, '5': 9, '10': 'storeID'},
    const {'1': 'filters', '3': 2, '4': 3, '5': 11, '6': '.api.pb.ListenRequest.Filter', '10': 'filters'},
  ],
  '3': const [ListenRequest_Filter$json],
};

const ListenRequest_Filter$json = const {
  '1': 'Filter',
  '2': const [
    const {'1': 'modelName', '3': 1, '4': 1, '5': 9, '10': 'modelName'},
    const {'1': 'entityID', '3': 2, '4': 1, '5': 9, '10': 'entityID'},
    const {'1': 'action', '3': 3, '4': 1, '5': 14, '6': '.api.pb.ListenRequest.Filter.Action', '10': 'action'},
  ],
  '4': const [ListenRequest_Filter_Action$json],
};

const ListenRequest_Filter_Action$json = const {
  '1': 'Action',
  '2': const [
    const {'1': 'ALL', '2': 0},
    const {'1': 'CREATE', '2': 1},
    const {'1': 'SAVE', '2': 2},
    const {'1': 'DELETE', '2': 3},
  ],
};

const ListenReply$json = const {
  '1': 'ListenReply',
  '2': const [
    const {'1': 'modelName', '3': 1, '4': 1, '5': 9, '10': 'modelName'},
    const {'1': 'entityID', '3': 2, '4': 1, '5': 9, '10': 'entityID'},
    const {'1': 'action', '3': 3, '4': 1, '5': 14, '6': '.api.pb.ListenReply.Action', '10': 'action'},
    const {'1': 'entity', '3': 4, '4': 1, '5': 12, '10': 'entity'},
  ],
  '4': const [ListenReply_Action$json],
};

const ListenReply_Action$json = const {
  '1': 'Action',
  '2': const [
    const {'1': 'CREATE', '2': 0},
    const {'1': 'SAVE', '2': 1},
    const {'1': 'DELETE', '2': 2},
  ],
};

