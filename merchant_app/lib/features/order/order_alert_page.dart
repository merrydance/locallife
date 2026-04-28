import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:merchant_app/config/theme.dart';
import 'package:merchant_app/models/order.dart';
import 'package:merchant_app/models/push_message.dart';
import 'package:merchant_app/features/order/order_provider.dart';
import 'package:merchant_app/features/printer/printer_provider.dart';
import 'package:merchant_app/features/settings/notification_settings_provider.dart';
import 'package:merchant_app/widgets/merchant_primary_button.dart';
import 'package:merchant_app/widgets/merchant_secondary_button.dart';

class OrderAlertPage extends ConsumerWidget {
  final PushMessage message;

  const OrderAlertPage({super.key, required this.message});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final isProcessing = ref.watch(
      orderProvider.select(
        (state) => state.actionInFlightOrderIds.contains(message.orderId),
      ),
    );

    return Scaffold(
      body: Container(
        decoration: const BoxDecoration(
          gradient: LinearGradient(
            begin: Alignment.topCenter,
            end: Alignment.bottomCenter,
            colors: [AppColors.tertiary, Color(0xFFC43B2F)],
          ),
        ),
        child: SafeArea(
          child: Padding(
            padding: const EdgeInsets.all(AppSpacing.xl),
            child: Column(
              children: [
                const Spacer(),
                const Icon(
                  Icons.notifications_active_rounded,
                  size: 96,
                  color: Colors.white,
                ),
                const SizedBox(height: AppSpacing.xl),
                const Text(
                  '您有新的订单',
                  style: TextStyle(
                    color: Colors.white,
                    fontSize: 30,
                    fontWeight: FontWeight.w700,
                  ),
                ),
                const SizedBox(height: AppSpacing.md),
                Text(
                  '请尽快确认是否接单，避免超时影响顾客体验。',
                  textAlign: TextAlign.center,
                  style: TextStyle(
                    color: Colors.white.withValues(alpha: 0.86),
                    height: 1.5,
                  ),
                ),
                const SizedBox(height: AppSpacing.xl),
                Container(
                  width: double.infinity,
                  padding: const EdgeInsets.all(AppSpacing.xxl),
                  decoration: BoxDecoration(
                    color: Colors.white.withValues(alpha: 0.18),
                    borderRadius: BorderRadius.circular(AppRadius.xl),
                  ),
                  child: Column(
                    children: [
                      Text(
                        '¥ ${message.amount.toStringAsFixed(2)}',
                        style: const TextStyle(
                          color: Colors.white,
                          fontSize: 46,
                          fontWeight: FontWeight.w700,
                        ),
                      ),
                      const SizedBox(height: AppSpacing.sm),
                      Text(
                        message.shopName,
                        textAlign: TextAlign.center,
                        style: TextStyle(
                          color: Colors.white.withValues(alpha: 0.88),
                          fontSize: 16,
                        ),
                      ),
                      const SizedBox(height: AppSpacing.md),
                      Text(
                        '订单号 ${message.displayOrderNumber}',
                        style: TextStyle(
                          color: Colors.white.withValues(alpha: 0.72),
                          fontSize: 13,
                        ),
                      ),
                      const SizedBox(height: AppSpacing.lg),
                      _AlertOrderSummary(message: message),
                    ],
                  ),
                ),
                const Spacer(),
                Theme(
                  data: Theme.of(context).copyWith(
                    outlinedButtonTheme: OutlinedButtonThemeData(
                      style: OutlinedButton.styleFrom(
                        minimumSize: const Size(0, 56),
                        side: BorderSide(
                          color: Colors.white.withValues(alpha: 0.5),
                        ),
                        foregroundColor: Colors.white,
                        shape: RoundedRectangleBorder(
                          borderRadius: BorderRadius.circular(AppRadius.lg),
                        ),
                      ),
                    ),
                    elevatedButtonTheme: ElevatedButtonThemeData(
                      style: ElevatedButton.styleFrom(
                        minimumSize: const Size(0, 56),
                        backgroundColor: Colors.white,
                        foregroundColor: AppColors.tertiary,
                        shape: RoundedRectangleBorder(
                          borderRadius: BorderRadius.circular(AppRadius.lg),
                        ),
                      ),
                    ),
                  ),
                  child: Row(
                    children: [
                      Expanded(
                        child: MerchantSecondaryButton(
                          expand: true,
                          label: '稍后处理',
                          onPressed: isProcessing
                              ? null
                              : () => Navigator.pop(context),
                        ),
                      ),
                      const SizedBox(width: AppSpacing.md),
                      Expanded(
                        child: MerchantPrimaryButton(
                          expand: true,
                          label: isProcessing ? '正在接单...' : '立即接单',
                          isLoading: isProcessing,
                          onPressed: isProcessing
                              ? null
                              : () async {
                                  final success = await ref
                                      .read(orderProvider.notifier)
                                      .acceptOrder(message.orderId);
                                  if (success) {
                                    final notificationSettings = ref.read(
                                      notificationSettingsProvider,
                                    );
                                    final printerState = ref.read(
                                      printerProvider,
                                    );
                                    final hydratedOrder = await ref
                                        .read(orderProvider.notifier)
                                        .fetchOrderDetail(message.orderId);
                                    await ref
                                        .read(orderProvider.notifier)
                                        .fetchOrders();
                                    if (notificationSettings
                                            .autoPrintAfterAcceptEnabled &&
                                        printerState.connectedDevice != null) {
                                      if (hydratedOrder != null) {
                                        await ref
                                            .read(printerProvider.notifier)
                                            .printAcceptedOrder(
                                              hydratedOrder,
                                              shopName: message.shopName,
                                            );
                                      }
                                    }
                                  }
                                  if (context.mounted) {
                                    ScaffoldMessenger.of(context).showSnackBar(
                                      SnackBar(
                                        content: Text(
                                          success ? '接单成功' : '接单失败，请稍后再试',
                                        ),
                                      ),
                                    );
                                  }
                                  if (success && context.mounted) {
                                    Navigator.pop(context);
                                  }
                                },
                        ),
                      ),
                    ],
                  ),
                ),
                const SizedBox(height: AppSpacing.lg),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class _AlertOrderSummary extends StatelessWidget {
  final PushMessage message;

  const _AlertOrderSummary({required this.message});

  @override
  Widget build(BuildContext context) {
    final note = message.note?.trim() ?? '';
    if (message.itemsLoadFailed) {
      return const _AlertMutedText('订单明细正在自动同步，请稍候。');
    }
    if (message.items.isEmpty && note.isEmpty) {
      return const SizedBox.shrink();
    }

    final visibleItems = message.items.take(3).toList();
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        ...visibleItems.map((item) => _AlertItemLine(item: item)),
        if (message.items.length > visibleItems.length)
          _AlertMutedText(
            '还有 ${message.items.length - visibleItems.length} 个商品，请查看详情',
          ),
        if (note.isNotEmpty) ...[
          const SizedBox(height: AppSpacing.sm),
          _AlertMutedText('备注：$note'),
        ],
      ],
    );
  }
}

class _AlertItemLine extends StatelessWidget {
  final OrderItem item;

  const _AlertItemLine({required this.item});

  @override
  Widget build(BuildContext context) {
    final specsText = item.specsText.trim();
    return Padding(
      padding: const EdgeInsets.only(bottom: AppSpacing.sm),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            '${item.name} x${item.quantity}',
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: TextStyle(
              color: Colors.white.withValues(alpha: 0.92),
              fontWeight: FontWeight.w600,
            ),
          ),
          if (specsText.isNotEmpty)
            Text(
              specsText,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(
                color: Colors.white.withValues(alpha: 0.72),
                fontSize: 12,
              ),
            ),
        ],
      ),
    );
  }
}

class _AlertMutedText extends StatelessWidget {
  final String text;

  const _AlertMutedText(this.text);

  @override
  Widget build(BuildContext context) {
    return Text(
      text,
      maxLines: 2,
      overflow: TextOverflow.ellipsis,
      style: TextStyle(
        color: Colors.white.withValues(alpha: 0.74),
        height: 1.35,
      ),
    );
  }
}
