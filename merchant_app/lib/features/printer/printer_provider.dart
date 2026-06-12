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
  final bool isScanning;
  final List<ScanResult> scanResults;
  final BluetoothDevice? connectedDevice;
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
    BluetoothDevice? connectedDevice,
    bool? isConnecting,
    String? error,
  }) {
    return PrinterState(
      isScanning: isScanning ?? this.isScanning,
      scanResults: scanResults ?? this.scanResults,
      connectedDevice: connectedDevice ?? this.connectedDevice,
      isConnecting: isConnecting ?? this.isConnecting,
      error: error, // Can be null
    );
  }
}

class PrinterNotifier extends StateNotifier<PrinterState> {
  StreamSubscription<List<ScanResult>>? _scanSubscription;
  StreamSubscription<BluetoothConnectionState>? _connectionSubscription;
  static const String _savedDeviceIdKey = 'saved_printer_id';

  PrinterNotifier() : super(PrinterState()) {
    _init();
  }

  Future<void> _init() async {
    // Check if adapter is on
    if (await FlutterBluePlus.adapterState.first == BluetoothAdapterState.on) {
      _tryConnectSavedDevice();
    }

    _scanSubscription = FlutterBluePlus.scanResults.listen((results) {
      state = state.copyWith(scanResults: results);
    });
  }

  Future<void> _tryConnectSavedDevice() async {
    final prefs = await SharedPreferences.getInstance();
    final savedId = prefs.getString(_savedDeviceIdKey);
    if (savedId == null) return;

    // We can't connect by ID directly if it's not cached in the system easily,
    // but in FlutterBluePlus we can try `BluetoothDevice.fromId` and connect.
    try {
      final device = BluetoothDevice.fromId(savedId);
      await connectToDevice(device);
    } catch (e) {
      debugPrint('Could not auto-connect: $e');
    }
  }

  Future<void> startScan() async {
    if (state.isScanning) return;

    // Check adapter state
    if (await FlutterBluePlus.adapterState.first != BluetoothAdapterState.on) {
      state = state.copyWith(error: "请先开启蓝牙");
      return;
    }

    state = state.copyWith(isScanning: true, scanResults: [], error: null);
    try {
      await FlutterBluePlus.startScan(timeout: const Duration(seconds: 10));
      // FlutterBluePlus will stop automatically after timeout
      Future.delayed(const Duration(seconds: 10), () {
        if (mounted) state = state.copyWith(isScanning: false);
      });
    } catch (e) {
      state = state.copyWith(isScanning: false, error: e.toString());
    }
  }

  Future<void> stopScan() async {
    await FlutterBluePlus.stopScan();
    state = state.copyWith(isScanning: false);
  }

  Future<void> connectToDevice(BluetoothDevice device) async {
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
          state = state.copyWith(connectedDevice: null);
        }
      });

      // Save ID for auto reconnect
      final prefs = await SharedPreferences.getInstance();
      await prefs.setString(_savedDeviceIdKey, device.remoteId.str);

      state = state.copyWith(connectedDevice: device, isConnecting: false);
    } catch (e) {
      state = state.copyWith(isConnecting: false, error: "连接失败: $e");
    }
  }

  Future<void> disconnect() async {
    if (state.connectedDevice != null) {
      await state.connectedDevice!.disconnect();
      final prefs = await SharedPreferences.getInstance();
      await prefs.remove(_savedDeviceIdKey);
      state = state.copyWith(connectedDevice: null);
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
      state = state.copyWith(error: "打印失败: $e");
      return false;
    }
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
