import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:merchant_app/config/theme.dart';
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
      orderProvider.select((state) => state.actionInFlightOrderIds.contains(message.orderId)),
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
                          onPressed: isProcessing ? null : () => Navigator.pop(context),
                        ),
                      ),
                      const SizedBox(width: AppSpacing.md),
                      Expanded(
                        child: MerchantPrimaryButton(
                          expand: true,
                          label: isProcessing ? '正在接单...' : '立即接单',
                          isLoading: isProcessing,
                          onPressed: isProcessing ? null : () async {
                            final success = await ref
                                .read(orderProvider.notifier)
                                .acceptOrder(message.orderId);
                            if (success) {
                              final notificationSettings =
                                  ref.read(notificationSettingsProvider);
                              final printerState = ref.read(printerProvider);
                              await ref.read(orderProvider.notifier).fetchOrders();
                              if (notificationSettings.autoPrintAfterAcceptEnabled &&
                                  printerState.connectedDevice != null) {
                                await ref
                                    .read(printerProvider.notifier)
                                    .printReceipt(message);
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
