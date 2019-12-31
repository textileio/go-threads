///
//  Generated code. Do not modify.
//  source: api.proto
//
// @dart = 2.3
// ignore_for_file: camel_case_types,non_constant_identifier_names,library_prefixes,unused_import,unused_shown_name,return_of_invalid_type

// ignore_for_file: UNDEFINED_SHOWN_NAME,UNUSED_SHOWN_NAME
import 'dart:core' as $core;
import 'package:protobuf/protobuf.dart' as $pb;

class ListenRequest_Filter_Action extends $pb.ProtobufEnum {
  static const ListenRequest_Filter_Action ALL = ListenRequest_Filter_Action._(0, 'ALL');
  static const ListenRequest_Filter_Action CREATE = ListenRequest_Filter_Action._(1, 'CREATE');
  static const ListenRequest_Filter_Action SAVE = ListenRequest_Filter_Action._(2, 'SAVE');
  static const ListenRequest_Filter_Action DELETE = ListenRequest_Filter_Action._(3, 'DELETE');

  static const $core.List<ListenRequest_Filter_Action> values = <ListenRequest_Filter_Action> [
    ALL,
    CREATE,
    SAVE,
    DELETE,
  ];

  static final $core.Map<$core.int, ListenRequest_Filter_Action> _byValue = $pb.ProtobufEnum.initByValue(values);
  static ListenRequest_Filter_Action valueOf($core.int value) => _byValue[value];

  const ListenRequest_Filter_Action._($core.int v, $core.String n) : super(v, n);
}

class ListenReply_Action extends $pb.ProtobufEnum {
  static const ListenReply_Action CREATE = ListenReply_Action._(0, 'CREATE');
  static const ListenReply_Action SAVE = ListenReply_Action._(1, 'SAVE');
  static const ListenReply_Action DELETE = ListenReply_Action._(2, 'DELETE');

  static const $core.List<ListenReply_Action> values = <ListenReply_Action> [
    CREATE,
    SAVE,
    DELETE,
  ];

  static final $core.Map<$core.int, ListenReply_Action> _byValue = $pb.ProtobufEnum.initByValue(values);
  static ListenReply_Action valueOf($core.int value) => _byValue[value];

  const ListenReply_Action._($core.int v, $core.String n) : super(v, n);
}

