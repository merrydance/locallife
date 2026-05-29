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
  static const List<int> _smallFontOn = [27, 77, 1];
  static const List<int> _smallFontOff = [27, 77, 0];
  static const List<int> _lineFeed = [10];

  /// Generate raw ESC/POS bytes for an order receipt using UTF-8 or GBK.
  /// Standard ESC/POS uses GBK for Chinese, but many modern cheap bluetooth
  /// printers support UTF-8 if properly converted, or GB18030.
  /// For simplicity, we assume UTF-8 is supported or it's a generic text transmission.
  /// (In a production app, we would use a charset conversion library for GBK).
  static List<int> generateOrderReceipt(PushMessage message) {
    if (message.itemsLoadFailed) {
      throw StateError('订单明细未完成同步，不能打印小票');
    }
    final feeBreakdown = message.feeBreakdown;
    if (feeBreakdown == null) {
      throw StateError('订单收款账单仍在同步，暂不打印小票');
    }

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

    if (message.items.isEmpty) {
      bytes.addAll(_encode("商品明细: 暂无\n"));
    } else {
      bytes.addAll(_boldOn);
      bytes.addAll(_encode("商品明细\n"));
      bytes.addAll(_boldOff);
      for (final item in message.items) {
        bytes.addAll(_encode("${item.name} x${item.quantity}\n"));
        if (item.specsText.trim().isNotEmpty) {
          bytes.addAll(_encode("  规格: ${item.specsText.trim()}\n"));
        }
        bytes.addAll(
          _encode(
            "  单价: ¥${item.price.toStringAsFixed(2)}  小计: ¥${item.lineTotal.toStringAsFixed(2)}\n",
          ),
        );
      }
    }

    if ((message.note ?? '').trim().isNotEmpty) {
      bytes.addAll(_encode("备注: ${message.note!.trim()}\n"));
    }
    bytes.addAll(_encode("--------------------------------\n"));
    bytes.addAll(_boldOn);
    bytes.addAll(
      _encode(
        "用户实付: ${_formatCents(feeBreakdown.customerPayableAmountCents)}\n",
      ),
    );
    bytes.addAll(_boldOff);
    bytes.addAll(_encode("商户账单\n"));
    bytes.addAll(
      _encode("菜品合计: ${_formatCents(feeBreakdown.foodPayableAmountCents)}\n"),
    );
    bytes.addAll(_smallFontOn);
    bytes.addAll(
      _encode(
        "- 平台服务费: ${_formatCents(feeBreakdown.platformServiceFeeAmountCents)}\n",
      ),
    );
    bytes.addAll(
      _encode(
        "- 支付通道费: ${_formatCents(feeBreakdown.paymentChannelFeeAmountCents)}\n",
      ),
    );
    bytes.addAll(_smallFontOff);
    bytes.addAll(
      _encode(
        "商户实收: ${_formatCents(feeBreakdown.merchantReceivableAmountCents)}\n",
      ),
    );
    final riderGrossAmount = feeBreakdown.riderGrossAmountCents > 0
        ? feeBreakdown.riderGrossAmountCents
        : feeBreakdown.deliveryFeeAmountCents;
    if (riderGrossAmount > 0 ||
        feeBreakdown.riderPaymentFeeCents > 0 ||
        feeBreakdown.riderNetEarningsCents > 0) {
      bytes.addAll(_encode("骑手账单\n"));
    }
    if (riderGrossAmount > 0) {
      bytes.addAll(_encode("代取费: ${_formatCents(riderGrossAmount)}\n"));
    }
    if (feeBreakdown.riderPaymentFeeCents > 0 ||
        feeBreakdown.riderNetEarningsCents > 0) {
      bytes.addAll(_smallFontOn);
      bytes.addAll(
        _encode(
          "- 支付通道费: ${_formatCents(feeBreakdown.riderPaymentFeeCents)}\n",
        ),
      );
      bytes.addAll(_smallFontOff);
      bytes.addAll(
        _encode("骑手实收: ${_formatCents(feeBreakdown.riderNetEarningsCents)}\n"),
      );
    }
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

  static String _formatCents(int cents) {
    return "¥${(cents / 100.0).toStringAsFixed(2)}";
  }
}
