import 'dart:async';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:merchant_app/core/network/api_client.dart';
import 'package:merchant_app/features/table/models/table_model.dart';
import 'package:merchant_app/features/table/providers/table_provider.dart';
import 'package:merchant_app/features/table/repositories/table_repository.dart';

void main() {
  group('TableRepository contract', () {
    test('uses backend table endpoints and response envelopes', () async {
      final apiClient = _RecordingApiClient();
      final repository = TableRepository(apiClient);

      await repository.listTables(tableType: 'room');
      await repository.getTable(11);
      await repository.createTable(
        tableNo: 'A01',
        tableType: 'table',
        capacity: 4,
        description: '靠窗',
        minimumSpend: 10000,
        accessCode: '123456',
        tagIds: [7, 8],
      );
      await repository.updateTable(
        11,
        tableNo: 'B01',
        tableType: 'room',
        capacity: 8,
        description: '包间',
        minimumSpend: 20000,
        accessCode: '654321',
        tagIds: [9],
      );
      await repository.updateTableStatus(11, 'occupied');
      await repository.listTableImages(11);
      await repository.addTableImage(11, mediaAssetId: 501, isPrimary: true);
      await repository.setTablePrimaryImage(11, 9001);
      await repository.deleteTableImage(11, 9001);
      await repository.generateTableQRCode(11);
      await repository.deleteTable(11);

      expect(apiClient.calls, <_ApiCall>[
        _ApiCall.get('/tables', queryParameters: {'table_type': 'room'}),
        _ApiCall.get('/tables/11'),
        _ApiCall.post(
          '/tables',
          data: {
            'table_no': 'A01',
            'table_type': 'table',
            'capacity': 4,
            'description': '靠窗',
            'minimum_spend': 10000,
            'access_code': '123456',
            'tag_ids': [7, 8],
          },
        ),
        _ApiCall.patch(
          '/tables/11',
          data: {
            'table_no': 'B01',
            'table_type': 'room',
            'capacity': 8,
            'description': '包间',
            'minimum_spend': 20000,
            'access_code': '654321',
            'tag_ids': [9],
          },
        ),
        _ApiCall.patch('/tables/11/status', data: {'status': 'occupied'}),
        _ApiCall.get('/tables/11/images'),
        _ApiCall.post(
          '/tables/11/images',
          data: {'media_asset_id': 501, 'sort_order': 0, 'is_primary': true},
        ),
        _ApiCall.put('/tables/11/images/9001/primary'),
        _ApiCall.delete('/tables/11/images/9001'),
        _ApiCall.get('/tables/11/qrcode'),
        _ApiCall.delete('/tables/11'),
      ]);
    });
  });

  group('TableNotifier state flow', () {
    test(
      'loads, creates, updates, changes status, and deletes from backend truth',
      () async {
        final repository = _FakeTableRepository(
          initialTables: [_table(id: 1, tableNo: 'A01')],
          createdTable: _table(id: 2, tableNo: 'A02'),
          updatedTable: _table(id: 1, tableNo: 'B01', capacity: 6),
          statusTable: _table(
            id: 1,
            tableNo: 'B01',
            status: TableStatus.occupied,
          ),
        );
        final notifier = TableNotifier(repository);

        await notifier.fetchTables();
        expect(notifier.state.tables.map((table) => table.id), [1]);

        final createdId = await notifier.createTableAndReturnId(
          tableNo: 'A02',
          tableType: 'table',
          capacity: 4,
        );
        expect(createdId, 2);
        expect(notifier.state.tables.map((table) => table.id), [1, 2]);

        final updated = await notifier.updateTable(
          tableId: 1,
          tableNo: 'B01',
          capacity: 6,
        );
        expect(updated, isTrue);
        expect(notifier.state.tables.first.tableNo, 'B01');
        expect(notifier.state.tables.first.capacity, 6);

        final statusChanged = await notifier.updateTableStatus(1, 'occupied');
        expect(statusChanged, isTrue);
        expect(notifier.state.tables.first.status, TableStatus.occupied);
        expect(notifier.state.actionInFlightTableIds, isEmpty);

        final deleted = await notifier.deleteTable(2);
        expect(deleted, isTrue);
        expect(notifier.state.tables.map((table) => table.id), [1]);
      },
    );

    test(
      'single-flights duplicate status changes for the same table',
      () async {
        final repository = _FakeTableRepository(
          initialTables: [_table(id: 1)],
          statusCompleter: Completer<TableModel>(),
        );
        final notifier = TableNotifier(repository);
        await notifier.fetchTables();

        final first = notifier.updateTableStatus(1, 'occupied');
        final second = notifier.updateTableStatus(1, 'occupied');

        expect(repository.statusUpdateCalls, 1);
        expect(notifier.state.actionInFlightTableIds, contains(1));

        repository.completeStatusUpdate(
          _table(id: 1, status: TableStatus.occupied),
        );

        expect(await first, isTrue);
        expect(await second, isTrue);
        expect(repository.statusUpdateCalls, 1);
        expect(notifier.state.tables.single.status, TableStatus.occupied);
        expect(notifier.state.actionInFlightTableIds, isEmpty);
      },
    );

    test('patches only matching table status from websocket payload', () async {
      final notifier = TableNotifier(
        _FakeTableRepository(
          initialTables: [
            _table(id: 1, status: TableStatus.available),
            _table(id: 2, status: TableStatus.disabled),
          ],
        ),
      );
      await notifier.fetchTables();

      notifier.updateTableFromWebSocket({
        'id': 1,
        'table_no': 'A01',
        'status': 'occupied',
      });

      expect(notifier.state.tables[0].status, TableStatus.occupied);
      expect(notifier.state.tables[1].status, TableStatus.disabled);
    });
  });
}

TableModel _table({
  required int id,
  String tableNo = 'A01',
  TableStatus status = TableStatus.available,
  int capacity = 4,
}) {
  return TableModel(
    id: id,
    merchantId: 10,
    tableNo: tableNo,
    tableType: TableType.table,
    capacity: capacity,
    status: status,
  );
}

Map<String, dynamic> _tableJson({
  required int id,
  String tableNo = 'A01',
  String status = 'available',
  int capacity = 4,
}) {
  return {
    'id': id,
    'merchant_id': 10,
    'table_no': tableNo,
    'table_type': 'table',
    'capacity': capacity,
    'status': status,
  };
}

class _FakeTableRepository extends TableRepository {
  _FakeTableRepository({
    this.initialTables = const [],
    TableModel? createdTable,
    TableModel? updatedTable,
    TableModel? statusTable,
    Completer<TableModel>? statusCompleter,
  }) : createdTable = createdTable ?? _table(id: 2),
       updatedTable = updatedTable ?? _table(id: 1),
       statusTable = statusTable ?? _table(id: 1, status: TableStatus.occupied),
       _statusCompleter = statusCompleter,
       super(_NoopApiClient());

  final List<TableModel> initialTables;
  final TableModel createdTable;
  final TableModel updatedTable;
  final TableModel statusTable;
  final Completer<TableModel>? _statusCompleter;
  int statusUpdateCalls = 0;

  @override
  Future<List<TableModel>> listTables({String? tableType}) async {
    return initialTables;
  }

  @override
  Future<TableModel> createTable({
    required String tableNo,
    required String tableType,
    required int capacity,
    String? description,
    int? minimumSpend,
    String? accessCode,
    List<int>? tagIds,
  }) async {
    return createdTable;
  }

  @override
  Future<TableModel> updateTable(
    int tableId, {
    String? tableNo,
    String? tableType,
    int? capacity,
    String? description,
    int? minimumSpend,
    String? accessCode,
    String? status,
    List<int>? tagIds,
  }) async {
    return updatedTable;
  }

  @override
  Future<TableModel> updateTableStatus(int tableId, String status) {
    statusUpdateCalls += 1;
    final completer = _statusCompleter;
    if (completer != null) {
      return completer.future;
    }
    return Future<TableModel>.value(statusTable);
  }

  void completeStatusUpdate(TableModel table) {
    _statusCompleter?.complete(table);
  }

  @override
  Future<void> deleteTable(int tableId) async {}
}

class _RecordingApiClient implements ApiClient {
  final calls = <_ApiCall>[];

  @override
  Future<Response<dynamic>> get(
    String path, {
    Map<String, dynamic>? queryParameters,
    bool requiresAuth = true,
  }) async {
    calls.add(_ApiCall.get(path, queryParameters: queryParameters));
    final Object data;
    if (path == '/tables') {
      data = {
        'tables': [_tableJson(id: 11)],
      };
    } else if (path.endsWith('/images')) {
      data = {
        'images': [
          {
            'id': 9001,
            'table_id': 11,
            'media_asset_id': 501,
            'image_url': 'https://example.test/table.jpg',
            'sort_order': 0,
            'is_primary': true,
          },
        ],
      };
    } else if (path.endsWith('/qrcode')) {
      data = {
        'qr_code_url': 'https://example.test/qr.png',
        'table_no': 'A01',
        'merchant_id': 10,
      };
    } else {
      data = _tableJson(id: 11);
    }
    return _ok(path, data);
  }

  @override
  Future<Response<dynamic>> post(
    String path, {
    dynamic data,
    bool requiresAuth = true,
  }) async {
    calls.add(_ApiCall.post(path, data: data));
    if (path.endsWith('/images')) {
      return _ok(path, {
        'id': 9001,
        'table_id': 11,
        'media_asset_id': data['media_asset_id'],
        'image_url': 'https://example.test/table.jpg',
        'sort_order': data['sort_order'],
        'is_primary': data['is_primary'],
      });
    }
    return _ok(path, _tableJson(id: 12, tableNo: data['table_no'] as String));
  }

  @override
  Future<Response<dynamic>> patch(
    String path, {
    dynamic data,
    bool requiresAuth = true,
  }) async {
    calls.add(_ApiCall.patch(path, data: data));
    return _ok(
      path,
      _tableJson(
        id: 11,
        tableNo: data['table_no'] as String? ?? 'A01',
        status: data['status'] as String? ?? 'available',
        capacity: data['capacity'] as int? ?? 4,
      ),
    );
  }

  @override
  Future<Response<dynamic>> put(
    String path, {
    dynamic data,
    bool requiresAuth = true,
  }) async {
    calls.add(_ApiCall.put(path, data: data));
    return _ok(path, {});
  }

  @override
  Future<Response<dynamic>> delete(
    String path, {
    bool requiresAuth = true,
  }) async {
    calls.add(_ApiCall.delete(path));
    return _ok(path, {});
  }

  @override
  Future<Map<String, String?>?> refreshSessionTokens() {
    throw UnimplementedError();
  }
}

class _NoopApiClient implements ApiClient {
  @override
  Future<Response<dynamic>> get(
    String path, {
    Map<String, dynamic>? queryParameters,
    bool requiresAuth = true,
  }) {
    throw UnimplementedError();
  }

  @override
  Future<Response<dynamic>> post(
    String path, {
    dynamic data,
    bool requiresAuth = true,
  }) {
    throw UnimplementedError();
  }

  @override
  Future<Response<dynamic>> patch(
    String path, {
    dynamic data,
    bool requiresAuth = true,
  }) {
    throw UnimplementedError();
  }

  @override
  Future<Response<dynamic>> put(
    String path, {
    dynamic data,
    bool requiresAuth = true,
  }) {
    throw UnimplementedError();
  }

  @override
  Future<Response<dynamic>> delete(String path, {bool requiresAuth = true}) {
    throw UnimplementedError();
  }

  @override
  Future<Map<String, String?>?> refreshSessionTokens() {
    throw UnimplementedError();
  }
}

Response<dynamic> _ok(String path, Object? data) {
  return Response<dynamic>(
    requestOptions: RequestOptions(path: path),
    data: {'code': 0, 'message': 'ok', 'data': data},
  );
}

class _ApiCall {
  final String method;
  final String path;
  final Map<String, dynamic>? queryParameters;
  final dynamic data;

  const _ApiCall(this.method, this.path, {this.queryParameters, this.data});

  const _ApiCall.get(String path, {Map<String, dynamic>? queryParameters})
    : this('GET', path, queryParameters: queryParameters);

  const _ApiCall.post(String path, {dynamic data})
    : this('POST', path, data: data);

  const _ApiCall.put(String path, {dynamic data})
    : this('PUT', path, data: data);

  const _ApiCall.patch(String path, {dynamic data})
    : this('PATCH', path, data: data);

  const _ApiCall.delete(String path) : this('DELETE', path);

  @override
  bool operator ==(Object other) {
    return other is _ApiCall &&
        other.method == method &&
        other.path == path &&
        _deepEquals(other.queryParameters, queryParameters) &&
        _deepEquals(other.data, data);
  }

  @override
  int get hashCode => Object.hash(
    method,
    path,
    Object.hashAll(_flatten(queryParameters)),
    Object.hashAll(_flatten(data)),
  );

  @override
  String toString() {
    return '_ApiCall($method $path, queryParameters: $queryParameters, data: $data)';
  }
}

bool _deepEquals(Object? left, Object? right) {
  if (left is Map && right is Map) {
    if (left.length != right.length) return false;
    return left.keys.every((key) => _deepEquals(left[key], right[key]));
  }
  if (left is List && right is List) {
    if (left.length != right.length) return false;
    for (var i = 0; i < left.length; i += 1) {
      if (!_deepEquals(left[i], right[i])) return false;
    }
    return true;
  }
  return left == right;
}

Iterable<Object?> _flatten(Object? value) sync* {
  if (value is Map) {
    for (final key
        in value.keys.toList()..sort((a, b) => '$a'.compareTo('$b'))) {
      yield key;
      yield* _flatten(value[key]);
    }
  } else if (value is List) {
    for (final item in value) {
      yield* _flatten(item);
    }
  } else {
    yield value;
  }
}
