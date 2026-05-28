import 'package:flutter/foundation.dart';
import 'package:path/path.dart' as path;
import 'package:sqflite/sqflite.dart';

class MessageDeduplicator {
  static const _maxMemorySize = 500;
  static const _retention = Duration(hours: 24);

  MessageDeduplicator({bool persist = true}) : _persist = persist;

  MessageDeduplicator.memoryOnly() : _persist = false;

  final bool _persist;
  final _memoryCache = <String, DateTime>{};
  Database? _database;
  Future<void>? _initFuture;

  Future<void> ensureInitialized() {
    _initFuture ??= _initialize();
    return _initFuture!;
  }

  Future<void> _initialize() async {
    if (!_persist || kIsWeb) {
      // sqflite does not support Web, fallback to memory-only
      return;
    }
    final databasesPath = await getDatabasesPath();
    final databasePath = path.join(databasesPath, 'message_dedup.db');
    _database = await openDatabase(
      databasePath,
      version: 1,
      onCreate: (db, version) async {
        await db.execute('''
          CREATE TABLE processed_messages (
            message_id TEXT PRIMARY KEY,
            processed_at INTEGER NOT NULL
          )
        ''');
      },
    );
    await _cleanupExpired();
  }

  Future<bool> tryAccept(String messageId) async {
    return tryAcceptGroup([messageId]);
  }

  Future<bool> tryAcceptGroup(List<String> rawKeys) async {
    final keys = _normalizeKeys(rawKeys);
    if (keys.isEmpty) {
      return false;
    }
    if (!await isAccepted(keys)) {
      return false;
    }
    await markAccepted(keys);
    return true;
  }

  Future<bool> isAccepted(List<String> rawKeys) async {
    await ensureInitialized();

    final keys = _normalizeKeys(rawKeys);
    if (keys.isEmpty) {
      return false;
    }

    for (final key in keys) {
      if (_memoryCache.containsKey(key)) {
        return false;
      }
    }

    final now = DateTime.now();
    final cutoff = now.subtract(_retention).millisecondsSinceEpoch;
    final db = _database;
    if (db == null) {
      return true;
    }

    final placeholders = List.filled(keys.length, '?').join(', ');
    final existing = await db.query(
      'processed_messages',
      columns: ['message_id'],
      where: 'message_id IN ($placeholders) AND processed_at >= ?',
      whereArgs: [...keys, cutoff],
      limit: 1,
    );
    if (existing.isNotEmpty) {
      return false;
    }

    return true;
  }

  Future<void> markAccepted(List<String> rawKeys) async {
    await ensureInitialized();

    final keys = _normalizeKeys(rawKeys);
    if (keys.isEmpty) {
      return;
    }

    final now = DateTime.now();
    final db = _database;
    if (db == null) {
      for (final key in keys) {
        _remember(key, now);
      }
      return;
    }

    final batch = db.batch();
    for (final key in keys) {
      _remember(key, now);
      batch.insert('processed_messages', {
        'message_id': key,
        'processed_at': now.millisecondsSinceEpoch,
      }, conflictAlgorithm: ConflictAlgorithm.ignore);
    }
    await batch.commit(noResult: true);

    if (_memoryCache.length % 50 == 0) {
      await _cleanupExpired();
    }
  }

  static String messageKey(String messageId) => 'message:$messageId';

  static String orderKey(String orderId) => 'order:$orderId';

  List<String> _normalizeKeys(List<String> rawKeys) {
    return rawKeys
        .map((key) => key.trim())
        .where((key) => key.isNotEmpty)
        .toSet()
        .toList(growable: false);
  }

  void _remember(String messageId, DateTime timestamp) {
    _memoryCache[messageId] = timestamp;
    if (_memoryCache.length > _maxMemorySize) {
      final oldestKey = _memoryCache.entries
          .reduce(
            (left, right) => left.value.isBefore(right.value) ? left : right,
          )
          .key;
      _memoryCache.remove(oldestKey);
    }
  }

  Future<void> _cleanupExpired() async {
    final db = _database;
    if (db == null) {
      return;
    }

    final cutoff = DateTime.now().subtract(_retention).millisecondsSinceEpoch;
    await db.delete(
      'processed_messages',
      where: 'processed_at < ?',
      whereArgs: [cutoff],
    );
    _memoryCache.removeWhere(
      (_, processedAt) => processedAt.millisecondsSinceEpoch < cutoff,
    );
  }
}
