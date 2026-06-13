import 'dart:async';
import 'package:flutter/foundation.dart';
import 'package:flutter_blue_plus/flutter_blue_plus.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:merchant_app/core/print/esc_pos_utils.dart'; // We will create this
import 'package:merchant_app/models/order.dart';
import 'package:merchant_app/models/push_message.dart'; // For order details

final printerProvider = StateNotifierProvider<PrinterNotifier, PrinterState>((
  ref,
) {
  return PrinterNotifier();
});

class PrinterState {
  static const Object _unset = Object();

  final bool isScanning;
  final List<ScanResult> scanResults;
  final BluetoothPrinterDevice? connectedDevice;
  final bool isConnecting;
  final String? error;

  PrinterState({
    this.isScanning = false,
    this.scanResults = const [],
    this.connectedDevice,
    this.isConnecting = false,
    this.error,
  });

  PrinterState copyWith({
    bool? isScanning,
    List<ScanResult>? scanResults,
    Object? connectedDevice = _unset,
    bool? isConnecting,
    Object? error = _unset,
  }) {
    return PrinterState(
      isScanning: isScanning ?? this.isScanning,
      scanResults: scanResults ?? this.scanResults,
      connectedDevice: identical(connectedDevice, _unset)
          ? this.connectedDevice
          : connectedDevice as BluetoothPrinterDevice?,
      isConnecting: isConnecting ?? this.isConnecting,
      error: identical(error, _unset) ? this.error : error as String?,
    );
  }
}

abstract interface class BluetoothPrinterDevice {
  String get id;

  String get name;

  Stream<BluetoothConnectionState> get connectionState;

  Future<void> connect({required bool autoConnect});

  Future<void> disconnect();

  Future<List<BluetoothService>> discoverServices();
}

class FlutterBluePrinterDevice implements BluetoothPrinterDevice {
  FlutterBluePrinterDevice(this._device);

  final BluetoothDevice _device;

  @override
  String get id => _device.remoteId.str;

  @override
  String get name => _device.platformName;

  @override
  Stream<BluetoothConnectionState> get connectionState =>
      _device.connectionState;

  @override
  Future<void> connect({required bool autoConnect}) {
    return _device.connect(autoConnect: autoConnect);
  }

  @override
  Future<void> disconnect() {
    return _device.disconnect();
  }

  @override
  Future<List<BluetoothService>> discoverServices() {
    return _device.discoverServices();
  }
}

abstract interface class BluetoothPrinterPlatform {
  Stream<BluetoothAdapterState> get adapterState;

  Stream<List<ScanResult>> get scanResults;

  Future<void> startScan({required Duration timeout});

  Future<void> stopScan();

  BluetoothPrinterDevice deviceFromId(String id);
}

class FlutterBluePrinterPlatform implements BluetoothPrinterPlatform {
  @override
  Stream<BluetoothAdapterState> get adapterState =>
      FlutterBluePlus.adapterState;

  @override
  Stream<List<ScanResult>> get scanResults => FlutterBluePlus.scanResults;

  @override
  Future<void> startScan({required Duration timeout}) {
    return FlutterBluePlus.startScan(timeout: timeout);
  }

  @override
  Future<void> stopScan() {
    return FlutterBluePlus.stopScan();
  }

  @override
  BluetoothPrinterDevice deviceFromId(String id) {
    return FlutterBluePrinterDevice(BluetoothDevice.fromId(id));
  }
}

class PrinterNotifier extends StateNotifier<PrinterState> {
  StreamSubscription<List<ScanResult>>? _scanSubscription;
  StreamSubscription<BluetoothConnectionState>? _connectionSubscription;
  static const String _savedDeviceIdKey = 'saved_printer_id';
  final BluetoothPrinterPlatform _platform;
  final Future<SharedPreferences> Function() _preferencesProvider;
  late final Future<void> _initFuture;

  PrinterNotifier({
    BluetoothPrinterPlatform? platform,
    Future<SharedPreferences> Function()? preferencesProvider,
    bool autoInit = true,
  }) : _platform = platform ?? FlutterBluePrinterPlatform(),
       _preferencesProvider =
           preferencesProvider ?? SharedPreferences.getInstance,
       super(PrinterState()) {
    _initFuture = autoInit ? _init() : Future<void>.value();
  }

  Future<void> get initialized => _initFuture;

  Future<void> _init() async {
    // Check if adapter is on
    if (await _platform.adapterState.first == BluetoothAdapterState.on) {
      await _tryConnectSavedDevice();
    }

    _scanSubscription = _platform.scanResults.listen((results) {
      state = state.copyWith(scanResults: results);
    });
  }

  Future<void> _tryConnectSavedDevice() async {
    final prefs = await _preferencesProvider();
    final savedId = prefs.getString(_savedDeviceIdKey);
    if (savedId == null) return;

    try {
      final device = _platform.deviceFromId(savedId);
      await connectToPrinterDevice(device, isAutoReconnect: true);
    } catch (e) {
      debugPrint('Could not auto-connect: $e');
    }
  }

  Future<void> startScan() async {
    if (state.isScanning) return;

    // Check adapter state
    if (await _platform.adapterState.first != BluetoothAdapterState.on) {
      state = state.copyWith(error: "请先开启蓝牙");
      return;
    }

    state = state.copyWith(isScanning: true, scanResults: [], error: null);
    try {
      await _platform.startScan(timeout: const Duration(seconds: 10));
      // FlutterBluePlus will stop automatically after timeout
      Future.delayed(const Duration(seconds: 10), () {
        if (mounted) state = state.copyWith(isScanning: false);
      });
    } catch (e) {
      debugPrint('Could not scan printers: $e');
      state = state.copyWith(isScanning: false, error: '扫描失败，请确认蓝牙权限和设备状态后重试');
    }
  }

  Future<void> stopScan() async {
    await _platform.stopScan();
    state = state.copyWith(isScanning: false);
  }

  Future<void> connectToDevice(BluetoothDevice device) async {
    await connectToPrinterDevice(FlutterBluePrinterDevice(device));
  }

  Future<void> connectToPrinterDevice(
    BluetoothPrinterDevice device, {
    bool isAutoReconnect = false,
  }) async {
    state = state.copyWith(isConnecting: true, error: null);
    await stopScan();

    try {
      if (state.connectedDevice != null) {
        await disconnect();
      }

      await device.connect(autoConnect: false);

      _connectionSubscription?.cancel();
      _connectionSubscription = device.connectionState.listen((event) {
        if (event == BluetoothConnectionState.disconnected) {
          state = state.copyWith(
            connectedDevice: null,
            isConnecting: false,
            error: _printerDisconnectedMessage(device.name),
          );
        }
      });

      // Save ID for auto reconnect
      final prefs = await _preferencesProvider();
      await prefs.setString(_savedDeviceIdKey, device.id);

      state = state.copyWith(connectedDevice: device, isConnecting: false);
    } catch (e) {
      debugPrint('Could not connect printer: $e');
      state = state.copyWith(
        isConnecting: false,
        error: isAutoReconnect
            ? '上次连接的打印机暂时无法连接，请确认设备已开机并靠近手机后重新连接'
            : '连接失败，请确认打印机已开机并靠近手机后重试',
      );
    }
  }

  Future<void> disconnect() async {
    final connectedDevice = state.connectedDevice;
    if (connectedDevice != null) {
      await _connectionSubscription?.cancel();
      _connectionSubscription = null;
      await connectedDevice.disconnect();
      final prefs = await _preferencesProvider();
      await prefs.remove(_savedDeviceIdKey);
      state = state.copyWith(connectedDevice: null, error: null);
    }
  }

  Future<bool> printReceipt(PushMessage message) async {
    if (state.connectedDevice == null) {
      state = state.copyWith(error: "打印机未连接");
      return false;
    }
    if (message.itemsLoadFailed) {
      state = state.copyWith(error: "订单明细仍在同步，暂不打印小票");
      return false;
    }
    if (message.feeBreakdown == null) {
      state = state.copyWith(error: "订单收款账单仍在同步，暂不打印小票");
      return false;
    }

    try {
      // Find write characteristic
      List<BluetoothService> services = await state.connectedDevice!
          .discoverServices();
      BluetoothCharacteristic? writeCharacteristic;

      for (var service in services) {
        for (var characteristic in service.characteristics) {
          if (characteristic.properties.write ||
              characteristic.properties.writeWithoutResponse) {
            writeCharacteristic = characteristic;
            break;
          }
        }
        if (writeCharacteristic != null) break;
      }

      if (writeCharacteristic == null) {
        state = state.copyWith(error: "未找到打印机写入通道");
        return false;
      }

      final bytes = EscPosUtils.generateOrderReceipt(message);

      // Send bytes in chunks
      int chunkSize = 20;
      for (int i = 0; i < bytes.length; i += chunkSize) {
        int end = (i + chunkSize < bytes.length) ? i + chunkSize : bytes.length;
        await writeCharacteristic.write(
          bytes.sublist(i, end),
          withoutResponse: writeCharacteristic.properties.writeWithoutResponse,
        );
      }
      return true;
    } catch (e) {
      debugPrint('Could not print receipt: $e');
      state = state.copyWith(error: '打印失败，请检查打印机连接后重试');
      return false;
    }
  }

  String _printerDisconnectedMessage(String name) {
    final normalized = name.trim();
    if (normalized.isEmpty) {
      return '打印机连接已断开，请重新连接';
    }
    return '$normalized 连接已断开，请重新连接';
  }

  Future<bool> printAcceptedOrder(
    OrderModel order, {
    required String shopName,
  }) async {
    if (!order.hasReliableItems) {
      state = state.copyWith(error: "订单明细仍在同步，暂不打印小票");
      return false;
    }
    final normalizedShopName = shopName.trim().isNotEmpty
        ? shopName.trim()
        : '商户工作台';
    return printReceipt(
      PushMessage(
        messageId: 'accepted-${order.id}',
        orderId: order.id,
        orderNumber: order.orderNum,
        orderType: order.orderType,
        pickupCode: order.pickupCode,
        pickupCodeMasked: order.pickupCodeMasked,
        title: '订单已接单',
        content: '订单已确认，准备打印小票',
        amount: order.amount,
        shopName: normalizedShopName,
        note: order.note,
        items: order.items,
        itemsLoadFailed: order.itemsLoadFailed,
        feeBreakdown: order.feeBreakdown,
      ),
    );
  }

  @override
  void dispose() {
    _scanSubscription?.cancel();
    _connectionSubscription?.cancel();
    super.dispose();
  }
}
