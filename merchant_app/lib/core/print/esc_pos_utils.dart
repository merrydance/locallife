import 'dart:convert';
import 'package:merchant_app/models/push_message.dart';

class EscPosUtils {
  static const List<int> _initPrinter = [27, 64];
  static const List<int> _alignCenter = [27, 97, 1];
  static const List<int> _alignLeft = [27, 97, 0];
  static const List<int> _boldOn = [27, 69, 1];
  static const List<int> _boldOff = [27, 69, 0];
  static const List<int> _doubleHeightOn = [29, 33, 1];
  static const List<int> _doubleHeightOff = [29, 33, 0];
  static const List<int> _lineFeed = [10];

  /// Generate raw ESC/POS bytes for an order receipt using UTF-8 or GBK.
  /// Standard ESC/POS uses GBK for Chinese, but many modern cheap bluetooth
  /// printers support UTF-8 if properly converted, or GB18030.
  /// For simplicity, we assume UTF-8 is supported or it's a generic text transmission.
  /// (In a production app, we would use a charset conversion library for GBK).
  static List<int> generateOrderReceipt(PushMessage message) {
    List<int> bytes = [];

    // Init printer
    bytes.addAll(_initPrinter);

    // Title
    bytes.addAll(_alignCenter);
    bytes.addAll(_doubleHeightOn);
    bytes.addAll(_boldOn);
    bytes.addAll(_encode("乐客来福 外卖订单\n"));
    bytes.addAll(_boldOff);
    bytes.addAll(_doubleHeightOff);
    bytes.addAll(_lineFeed);

    // Order Info
    bytes.addAll(_alignLeft);
    bytes.addAll(_encode("所属店铺: ${message.shopName}\n"));
    bytes.addAll(_encode("订单编号: ${message.displayOrderNumber}\n"));
    bytes.addAll(_encode("--------------------------------\n"));

    // Items (PushMessage currently doesn't carry items array, so we just show amount)
    bytes.addAll(_boldOn);
    bytes.addAll(_encode("订单总额: ¥${message.amount.toStringAsFixed(2)}\n"));
    bytes.addAll(_boldOff);
    bytes.addAll(_encode("--------------------------------\n"));

    // Footer
    bytes.addAll(_alignCenter);
    bytes.addAll(_encode("感谢使用乐客来福商户版\n"));
    bytes.addAll(_lineFeed);
    bytes.addAll(_lineFeed);
    bytes.addAll(_lineFeed);

    return bytes;
  }

  static List<int> _encode(String text) {
    return utf8.encode(text);
  }
}
