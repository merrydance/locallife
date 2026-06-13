import 'dart:async';

import 'package:flutter_blue_plus/flutter_blue_plus.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:merchant_app/features/printer/printer_provider.dart';
import 'package:merchant_app/models/push_message.dart';
import 'package:merchant_app/models/order.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test('auto reconnects the saved printer when bluetooth is on', () async {
    SharedPreferences.setMockInitialValues(<String, Object>{
      'saved_printer_id': 'printer-1',
    });
    final device = _FakeBluetoothPrinterDevice(id: 'printer-1', name: '前台打印机');
    final platform = _FakeBluetoothPrinterPlatform(
      adapterState: BluetoothAdapterState.on,
      devices: <String, BluetoothPrinterDevice>{'printer-1': device},
    );

    final notifier = PrinterNotifier(platform: platform);
    addTearDown(notifier.dispose);

    await notifier.initialized;

    expect(device.connectCalls, 1);
    expect(notifier.state.connectedDevice?.id, 'printer-1');
    expect(notifier.state.error, isNull);
  });

  test(
    'auto reconnect failure keeps a merchant-facing recovery message',
    () async {
      SharedPreferences.setMockInitialValues(<String, Object>{
        'saved_printer_id': 'printer-2',
      });
      final device = _FakeBluetoothPrinterDevice(
        id: 'printer-2',
        name: '后厨打印机',
        connectError: StateError('platform GATT 133'),
      );
      final platform = _FakeBluetoothPrinterPlatform(
        adapterState: BluetoothAdapterState.on,
        devices: <String, BluetoothPrinterDevice>{'printer-2': device},
      );

      final notifier = PrinterNotifier(platform: platform);
      addTearDown(notifier.dispose);

      await notifier.initialized;

      expect(device.connectCalls, 1);
      expect(notifier.state.connectedDevice, isNull);
      expect(notifier.state.error, '上次连接的打印机暂时无法连接，请确认设备已开机并靠近手机后重新连接');
      expect(notifier.state.error, isNot(contains('GATT')));
    },
  );

  test(
    'unexpected bluetooth disconnect clears connected device and shows copy',
    () async {
      SharedPreferences.setMockInitialValues(<String, Object>{});
      final device = _FakeBluetoothPrinterDevice(
        id: 'printer-3',
        name: '前台打印机',
      );
      final notifier = PrinterNotifier(
        platform: _FakeBluetoothPrinterPlatform(
          adapterState: BluetoothAdapterState.on,
          devices: <String, BluetoothPrinterDevice>{'printer-3': device},
        ),
        autoInit: false,
      );
      addTearDown(notifier.dispose);

      await notifier.connectToPrinterDevice(device);
      device.emitDisconnected();
      await Future<void>.delayed(Duration.zero);

      expect(notifier.state.connectedDevice, isNull);
      expect(notifier.state.error, '前台打印机 连接已断开，请重新连接');
    },
  );

  test('manual connect failure uses stable Chinese recovery copy', () async {
    SharedPreferences.setMockInitialValues(<String, Object>{});
    final device = _FakeBluetoothPrinterDevice(
      id: 'printer-4',
      name: '前台打印机',
      connectError: StateError('platform GATT 133'),
    );
    final notifier = PrinterNotifier(
      platform: _FakeBluetoothPrinterPlatform(
        adapterState: BluetoothAdapterState.on,
        devices: <String, BluetoothPrinterDevice>{'printer-4': device},
      ),
      autoInit: false,
    );
    addTearDown(notifier.dispose);

    await notifier.connectToPrinterDevice(device);

    expect(notifier.state.connectedDevice, isNull);
    expect(notifier.state.error, '连接失败，请确认打印机已开机并靠近手机后重试');
    expect(notifier.state.error, isNot(contains('GATT')));
  });

  test('scan failure uses stable Chinese recovery copy', () async {
    SharedPreferences.setMockInitialValues(<String, Object>{});
    final notifier = PrinterNotifier(
      platform: _FakeBluetoothPrinterPlatform(
        adapterState: BluetoothAdapterState.on,
        devices: const <String, BluetoothPrinterDevice>{},
        startScanError: StateError('platform scan blew up'),
      ),
      autoInit: false,
    );
    addTearDown(notifier.dispose);

    await notifier.startScan();

    expect(notifier.state.isScanning, isFalse);
    expect(notifier.state.error, '扫描失败，请确认蓝牙权限和设备状态后重试');
    expect(notifier.state.error, isNot(contains('platform scan')));
  });

  test('receipt print failure uses stable Chinese recovery copy', () async {
    SharedPreferences.setMockInitialValues(<String, Object>{});
    final device = _FakeBluetoothPrinterDevice(
      id: 'printer-5',
      name: '前台打印机',
      discoverServicesError: StateError('platform GATT 133'),
    );
    final notifier = PrinterNotifier(
      platform: _FakeBluetoothPrinterPlatform(
        adapterState: BluetoothAdapterState.on,
        devices: <String, BluetoothPrinterDevice>{'printer-5': device},
      ),
      autoInit: false,
    );
    addTearDown(notifier.dispose);
    await notifier.connectToPrinterDevice(device);

    final printed = await notifier.printReceipt(
      PushMessage(
        messageId: 'message-1',
        orderId: 'order-1',
        title: '订单已接单',
        content: '订单已确认',
        amount: 12,
        shopName: '测试门店',
        feeBreakdown: const OrderFeeBreakdown(),
      ),
    );

    expect(printed, isFalse);
    expect(notifier.state.error, '打印失败，请检查打印机连接后重试');
    expect(notifier.state.error, isNot(contains('GATT')));
  });
}

class _FakeBluetoothPrinterPlatform implements BluetoothPrinterPlatform {
  _FakeBluetoothPrinterPlatform({
    required BluetoothAdapterState adapterState,
    required this.devices,
    this.startScanError,
  }) : _adapterState = adapterState;

  final BluetoothAdapterState _adapterState;
  final Map<String, BluetoothPrinterDevice> devices;
  final Object? startScanError;
  bool stoppedScan = false;

  @override
  Stream<BluetoothAdapterState> get adapterState => Stream.value(_adapterState);

  @override
  Stream<List<ScanResult>> get scanResults =>
      const Stream<List<ScanResult>>.empty();

  @override
  BluetoothPrinterDevice deviceFromId(String id) => devices[id]!;

  @override
  Future<void> startScan({required Duration timeout}) async {
    final error = startScanError;
    if (error != null) {
      throw error;
    }
  }

  @override
  Future<void> stopScan() async {
    stoppedScan = true;
  }
}

class _FakeBluetoothPrinterDevice implements BluetoothPrinterDevice {
  _FakeBluetoothPrinterDevice({
    required this.id,
    required this.name,
    this.connectError,
    this.discoverServicesError,
  });

  @override
  final String id;

  @override
  final String name;

  final Object? connectError;
  final Object? discoverServicesError;
  final StreamController<BluetoothConnectionState> _connectionController =
      StreamController<BluetoothConnectionState>.broadcast();
  int connectCalls = 0;
  bool disconnected = false;

  @override
  Stream<BluetoothConnectionState> get connectionState =>
      _connectionController.stream;

  @override
  Future<void> connect({required bool autoConnect}) async {
    connectCalls += 1;
    final error = connectError;
    if (error != null) {
      throw error;
    }
  }

  @override
  Future<void> disconnect() async {
    disconnected = true;
    emitDisconnected();
  }

  @override
  Future<List<BluetoothService>> discoverServices() async {
    final error = discoverServicesError;
    if (error != null) {
      throw error;
    }
    return <BluetoothService>[];
  }

  void emitDisconnected() {
    _connectionController.add(BluetoothConnectionState.disconnected);
  }
}
