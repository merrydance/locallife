import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:merchant_app/config/theme.dart';
import 'package:merchant_app/models/order.dart';
import 'package:merchant_app/models/push_message.dart';
import 'package:merchant_app/features/order/order_acceptance_coordinator.dart';
import 'package:merchant_app/features/order/order_provider.dart';
import 'package:merchant_app/widgets/merchant_primary_button.dart';

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
    final displayFoodAmount =
        message.feeBreakdown?.foodPayableAmount ?? message.amount;
    final deliveryFeeAmount =
        message.feeBreakdown?.deliveryPayableAmount ?? 0.0;

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
          child: LayoutBuilder(
            builder: (context, constraints) {
              final minHeight = constraints.maxHeight > AppSpacing.xl * 2
                  ? constraints.maxHeight - AppSpacing.xl * 2
                  : 0.0;
              return SingleChildScrollView(
                child: Padding(
                  padding: const EdgeInsets.all(AppSpacing.xl),
                  child: ConstrainedBox(
                    constraints: BoxConstraints(minHeight: minHeight),
                    child: IntrinsicHeight(
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
                                  '¥ ${displayFoodAmount.toStringAsFixed(2)}',
                                  style: const TextStyle(
                                    color: Colors.white,
                                    fontSize: 46,
                                    fontWeight: FontWeight.w700,
                                  ),
                                ),
                                if (deliveryFeeAmount > 0) ...[
                                  const SizedBox(height: AppSpacing.xs),
                                  Text(
                                    '代取费另列 ¥${deliveryFeeAmount.toStringAsFixed(2)}',
                                    style: TextStyle(
                                      color: Colors.white.withValues(
                                        alpha: 0.78,
                                      ),
                                      fontSize: 13,
                                    ),
                                  ),
                                ],
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
                                    borderRadius: BorderRadius.circular(
                                      AppRadius.lg,
                                    ),
                                  ),
                                ),
                              ),
                              elevatedButtonTheme: ElevatedButtonThemeData(
                                style: ElevatedButton.styleFrom(
                                  minimumSize: const Size(0, 56),
                                  backgroundColor: Colors.white,
                                  foregroundColor: AppColors.tertiary,
                                  shape: RoundedRectangleBorder(
                                    borderRadius: BorderRadius.circular(
                                      AppRadius.lg,
                                    ),
                                  ),
                                ),
                              ),
                            ),
                            child: Column(
                              children: [
                                MerchantPrimaryButton(
                                  expand: true,
                                  label: isProcessing ? '正在接单...' : '立即接单',
                                  isLoading: isProcessing,
                                  onPressed: isProcessing
                                      ? null
                                      : () async {
                                          final success = await ref
                                              .read(
                                                orderAcceptanceCoordinatorProvider,
                                              )
                                              .acceptOrder(
                                                message.orderId,
                                                messageSnapshot: message,
                                                shopName: message.shopName,
                                              );
                                          if (context.mounted) {
                                            ScaffoldMessenger.of(
                                              context,
                                            ).showSnackBar(
                                              SnackBar(
                                                content: Text(
                                                  success
                                                      ? '接单成功'
                                                      : '接单失败，请稍后再试',
                                                ),
                                              ),
                                            );
                                          }
                                          if (success && context.mounted) {
                                            Navigator.pop(context);
                                          }
                                        },
                                ),
                                const SizedBox(height: AppSpacing.md),
                                Row(
                                  children: [
                                    Expanded(
                                      child: _AlertSecondaryButton(
                                        label: isProcessing ? '处理中...' : '拒单',
                                        onPressed: isProcessing
                                            ? null
                                            : () async {
                                                final reason =
                                                    await _showRejectReasonDialog(
                                                      context,
                                                    );
                                                if (reason == null ||
                                                    reason.trim().isEmpty ||
                                                    !context.mounted) {
                                                  return;
                                                }
                                                final success = await ref
                                                    .read(
                                                      orderProvider.notifier,
                                                    )
                                                    .rejectOrder(
                                                      message.orderId,
                                                      reason: reason.trim(),
                                                    );
                                                final refundMessage = ref
                                                    .read(
                                                      orderProvider.notifier,
                                                    )
                                                    .takeLastRejectRefundMessage();
                                                if (!context.mounted) {
                                                  return;
                                                }
                                                ScaffoldMessenger.of(
                                                  context,
                                                ).showSnackBar(
                                                  SnackBar(
                                                    content: Text(
                                                      success
                                                          ? (refundMessage ??
                                                                '已拒绝订单，退款状态请在订单详情中查看')
                                                          : ref
                                                                    .read(
                                                                      orderProvider,
                                                                    )
                                                                    .error ??
                                                                '拒单失败，请稍后再试',
                                                    ),
                                                  ),
                                                );
                                                if (success) {
                                                  await ref
                                                      .read(
                                                        orderProvider.notifier,
                                                      )
                                                      .fetchOrders();
                                                  if (context.mounted) {
                                                    Navigator.pop(context);
                                                  }
                                                }
                                              },
                                      ),
                                    ),
                                    const SizedBox(width: AppSpacing.md),
                                    Expanded(
                                      child: _AlertSecondaryButton(
                                        label: '稍后处理',
                                        onPressed: isProcessing
                                            ? null
                                            : () => Navigator.pop(context),
                                      ),
                                    ),
                                  ],
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
            },
          ),
        ),
      ),
    );
  }

  Future<String?> _showRejectReasonDialog(BuildContext context) async {
    final controller = TextEditingController();
    final formKey = GlobalKey<FormState>();

    final result = await showDialog<String>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('填写拒单原因'),
        content: Form(
          key: formKey,
          child: TextFormField(
            controller: controller,
            maxLines: 3,
            autofocus: true,
            decoration: const InputDecoration(hintText: '例如：菜品已售罄、设备故障、门店临时打烊'),
            validator: (value) {
              final text = value?.trim() ?? '';
              if (text.isEmpty) {
                return '请填写拒单原因';
              }
              if (text.length < 2) {
                return '拒单原因至少 2 个字';
              }
              return null;
            },
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(dialogContext),
            child: const Text('取消'),
          ),
          TextButton(
            onPressed: () {
              if (formKey.currentState?.validate() ?? false) {
                Navigator.pop(dialogContext, controller.text);
              }
            },
            child: const Text('确认拒单'),
          ),
        ],
      ),
    );

    controller.dispose();
    return result;
  }
}

class _AlertSecondaryButton extends StatelessWidget {
  final String label;
  final VoidCallback? onPressed;

  const _AlertSecondaryButton({required this.label, required this.onPressed});

  @override
  Widget build(BuildContext context) {
    return OutlinedButton(
      onPressed: onPressed,
      style: OutlinedButton.styleFrom(
        minimumSize: const Size(0, 56),
        foregroundColor: Colors.white,
        disabledForegroundColor: Colors.white.withValues(alpha: 0.48),
        side: BorderSide(color: Colors.white.withValues(alpha: 0.58)),
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(AppRadius.lg),
        ),
      ),
      child: Text(label),
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
