import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:merchant_app/config/theme.dart';
import 'package:merchant_app/features/auth/auth_provider.dart';
import 'package:merchant_app/features/order/order_provider.dart';
import 'package:merchant_app/features/printer/printer_provider.dart';
import 'package:merchant_app/features/settings/notification_settings_provider.dart';
import 'package:merchant_app/models/order.dart';
import 'package:merchant_app/widgets/merchant_content_shell.dart';
import 'package:merchant_app/widgets/merchant_primary_button.dart';
import 'package:merchant_app/widgets/merchant_secondary_button.dart';
import 'package:merchant_app/widgets/merchant_status_badge.dart';

class OrderDetailPage extends ConsumerWidget {
  final OrderModel order;

  const OrderDetailPage({super.key, required this.order});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final currentOrder =
        ref.watch(
          orderProvider.select(
            (state) => state.orders.cast<OrderModel?>().firstWhere(
              (candidate) => candidate?.id == order.id,
              orElse: () => null,
            ),
          ),
        ) ??
        order;

    return Scaffold(
      appBar: AppBar(title: const Text('订单详情')),
      body: SingleChildScrollView(
        child: MerchantContentShell(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              _buildStatusCard(context, currentOrder),
              const SizedBox(height: AppSpacing.lg),
              _buildCustomerInfoCard(context, currentOrder),
              const SizedBox(height: AppSpacing.lg),
              _buildItemsCard(context, currentOrder),
              if ((currentOrder.note ?? '').trim().isNotEmpty) ...[
                const SizedBox(height: AppSpacing.lg),
                _buildNoteCard(context, currentOrder),
              ],
              const SizedBox(height: AppSpacing.xl),
              _buildActionButtons(context, ref, currentOrder),
              const SizedBox(height: AppSpacing.xl),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildStatusCard(BuildContext context, OrderModel order) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(AppSpacing.xl),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        '订单 ${order.orderNum}',
                        style: const TextStyle(
                          fontSize: 20,
                          fontWeight: FontWeight.w700,
                        ),
                      ),
                      const SizedBox(height: AppSpacing.xs),
                      Text(
                        '下单时间 ${order.formattedDate}',
                        style: TextStyle(
                          color: Theme.of(context).colorScheme.onSurfaceVariant,
                        ),
                      ),
                    ],
                  ),
                ),
                MerchantStatusBadge(
                  label: order.status.label,
                  tone: _detailStatusToneFor(order.status),
                ),
              ],
            ),
            const SizedBox(height: AppSpacing.lg),
            Container(
              width: double.infinity,
              padding: const EdgeInsets.all(AppSpacing.lg),
              decoration: BoxDecoration(
                color: AppColors.surfaceLow,
                borderRadius: BorderRadius.circular(AppRadius.lg),
              ),
              child: Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      const Text(
                        '订单总额',
                        style: TextStyle(
                          color: AppColors.onSurfaceVariant,
                          fontSize: 12,
                        ),
                      ),
                      const SizedBox(height: AppSpacing.xs),
                      Text(
                        '¥${order.amount.toStringAsFixed(2)}',
                        style: const TextStyle(
                          fontSize: 24,
                          fontWeight: FontWeight.w700,
                          color: AppColors.secondary,
                        ),
                      ),
                    ],
                  ),
                  Column(
                    crossAxisAlignment: CrossAxisAlignment.end,
                    children: [
                      const Text(
                        '商品数量',
                        style: TextStyle(
                          color: AppColors.onSurfaceVariant,
                          fontSize: 12,
                        ),
                      ),
                      const SizedBox(height: AppSpacing.xs),
                      Text(
                        order.hasReliableItems
                            ? '${order.items.length} 件'
                            : '同步中',
                        style: const TextStyle(
                          fontSize: 18,
                          fontWeight: FontWeight.w700,
                        ),
                      ),
                    ],
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildCustomerInfoCard(BuildContext context, OrderModel order) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(AppSpacing.xl),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const Text(
              '联系人信息',
              style: TextStyle(fontWeight: FontWeight.w700, fontSize: 16),
            ),
            const SizedBox(height: AppSpacing.lg),
            _buildInfoRow(
              icon: Icons.person_outline,
              label: '联系人',
              value: order.userName?.isNotEmpty == true
                  ? order.userName!
                  : '匿名客户',
            ),
            const SizedBox(height: AppSpacing.md),
            _buildInfoRow(
              icon: Icons.phone_outlined,
              label: '联系电话',
              value: order.userPhone?.isNotEmpty == true
                  ? order.userPhone!
                  : '暂无电话',
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildItemsCard(BuildContext context, OrderModel order) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(AppSpacing.xl),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const Text(
              '商品清单',
              style: TextStyle(fontWeight: FontWeight.w700, fontSize: 16),
            ),
            const SizedBox(height: AppSpacing.lg),
            if (!order.hasReliableItems)
              _buildItemsStateText('订单明细正在自动同步，请稍候。')
            else if (order.items.isEmpty)
              _buildItemsStateText('暂无商品明细')
            else
              ...order.items.map((item) => _buildItemRow(item)),
            Row(
              mainAxisAlignment: MainAxisAlignment.end,
              children: [
                const Text('总计 ', style: TextStyle(fontSize: 16)),
                Text(
                  '¥${order.amount.toStringAsFixed(2)}',
                  style: const TextStyle(
                    fontSize: 24,
                    fontWeight: FontWeight.w700,
                    color: AppColors.secondary,
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildItemRow(OrderItem item) {
    final specsText = item.specsText.trim();
    return Padding(
      padding: const EdgeInsets.only(bottom: AppSpacing.md),
      child: Container(
        padding: const EdgeInsets.all(AppSpacing.lg),
        decoration: BoxDecoration(
          color: AppColors.surfaceLow,
          borderRadius: BorderRadius.circular(AppRadius.lg),
        ),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    item.name,
                    style: const TextStyle(
                      fontSize: 16,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                  if (specsText.isNotEmpty) ...[
                    const SizedBox(height: AppSpacing.xs),
                    Text(
                      specsText,
                      style: const TextStyle(
                        color: AppColors.onSurfaceVariant,
                        fontSize: 13,
                        height: 1.35,
                      ),
                    ),
                  ],
                  const SizedBox(height: AppSpacing.xs),
                  Text(
                    '数量 x${item.quantity}  单价 ¥${item.price.toStringAsFixed(2)}',
                    style: const TextStyle(
                      color: AppColors.onSurfaceVariant,
                      fontSize: 12,
                    ),
                  ),
                ],
              ),
            ),
            const SizedBox(width: AppSpacing.md),
            Text(
              '¥${item.lineTotal.toStringAsFixed(2)}',
              style: const TextStyle(fontWeight: FontWeight.w700, fontSize: 15),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildItemsStateText(String text) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(AppSpacing.lg),
      decoration: BoxDecoration(
        color: AppColors.surfaceLow,
        borderRadius: BorderRadius.circular(AppRadius.lg),
      ),
      child: Text(
        text,
        style: const TextStyle(color: AppColors.onSurfaceVariant, height: 1.45),
      ),
    );
  }

  Widget _buildNoteCard(BuildContext context, OrderModel order) {
    return Card(
      color: AppColors.warningSoft,
      child: Padding(
        padding: const EdgeInsets.all(AppSpacing.xl),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const Row(
              children: [
                Icon(Icons.notes_rounded, color: AppColors.secondary),
                SizedBox(width: 8),
                Text(
                  '顾客备注',
                  style: TextStyle(
                    fontWeight: FontWeight.w700,
                    fontSize: 16,
                    color: AppColors.secondary,
                  ),
                ),
              ],
            ),
            const SizedBox(height: AppSpacing.md),
            Text(
              order.note ?? '',
              style: const TextStyle(fontSize: 16, height: 1.45),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildActionButtons(
    BuildContext context,
    WidgetRef ref,
    OrderModel order,
  ) {
    final isAuthenticated = ref.watch(
      authProvider.select((state) => state.isAuthenticated),
    );
    final isProcessing = ref.watch(
      orderProvider.select(
        (state) => state.actionInFlightOrderIds.contains(order.id),
      ),
    );

    if (order.status != OrderStatus.pending || !isAuthenticated) {
      return const SizedBox.shrink();
    }

    return Row(
      children: [
        Expanded(
          child: MerchantSecondaryButton(
            expand: true,
            label: isProcessing ? '处理中...' : '暂不接单',
            onPressed: isProcessing
                ? null
                : () async {
                    final reason = await _showRejectReasonDialog(context);
                    if (reason == null ||
                        reason.trim().isEmpty ||
                        !context.mounted) {
                      return;
                    }
                    final success = await ref
                        .read(orderProvider.notifier)
                        .rejectOrder(order.id, reason: reason.trim());
                    if (!context.mounted) {
                      return;
                    }
                    ScaffoldMessenger.of(context).showSnackBar(
                      SnackBar(
                        content: Text(success ? '已拒绝订单' : '拒绝订单失败，请稍后再试'),
                      ),
                    );
                    if (success) {
                      Navigator.of(context).pop();
                    }
                  },
          ),
        ),
        const SizedBox(width: AppSpacing.md),
        Expanded(
          flex: 2,
          child: MerchantPrimaryButton(
            expand: true,
            label: isProcessing ? '正在接单...' : '立即接单',
            isLoading: isProcessing,
            onPressed: isProcessing
                ? null
                : () async {
                    final success = await ref
                        .read(orderProvider.notifier)
                        .acceptOrder(order.id);
                    if (success) {
                      final notificationSettings = ref.read(
                        notificationSettingsProvider,
                      );
                      final printerState = ref.read(printerProvider);
                      final merchantName =
                          ref.read(authProvider).merchantName ?? '商户工作台';
                      if (notificationSettings.autoPrintAfterAcceptEnabled &&
                          printerState.connectedDevice != null) {
                        await ref
                            .read(printerProvider.notifier)
                            .printAcceptedOrder(order, shopName: merchantName);
                      }
                    }
                    if (context.mounted && !success) {
                      ScaffoldMessenger.of(context).showSnackBar(
                        SnackBar(
                          content: Text(
                            ref.read(orderProvider).error ?? '接单失败，请稍后再试',
                          ),
                        ),
                      );
                    }
                    if (context.mounted && success) {
                      Navigator.of(context).pop();
                    }
                  },
          ),
        ),
      ],
    );
  }

  Widget _buildInfoRow({
    required IconData icon,
    required String label,
    required String value,
  }) {
    return Container(
      padding: const EdgeInsets.all(AppSpacing.lg),
      decoration: BoxDecoration(
        color: AppColors.surfaceLow,
        borderRadius: BorderRadius.circular(AppRadius.lg),
      ),
      child: Row(
        children: [
          Icon(icon, color: AppColors.onSurfaceVariant, size: 20),
          const SizedBox(width: AppSpacing.md),
          Text(
            '$label：',
            style: const TextStyle(
              color: AppColors.onSurfaceVariant,
              fontWeight: FontWeight.w600,
            ),
          ),
          Expanded(child: Text(value)),
        ],
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

MerchantStatusTone _detailStatusToneFor(OrderStatus status) {
  switch (status) {
    case OrderStatus.pending:
      return MerchantStatusTone.warning;
    case OrderStatus.cancelled:
      return MerchantStatusTone.danger;
    case OrderStatus.completed:
      return MerchantStatusTone.neutral;
    case OrderStatus.accepted:
    case OrderStatus.preparing:
    case OrderStatus.delivering:
      return MerchantStatusTone.positive;
  }
}
