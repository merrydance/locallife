import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:merchant_app/core/network/connectivity_provider.dart';
import 'package:merchant_app/core/network/ws_provider.dart';
import 'package:merchant_app/config/theme.dart';
import 'package:merchant_app/features/printer/printer_provider.dart';
import 'package:merchant_app/features/order/order_provider.dart';
import 'package:merchant_app/features/order/working_status_provider.dart';
import 'package:merchant_app/features/auth/auth_provider.dart';
import 'package:merchant_app/features/settings/notification_settings_provider.dart';
import 'package:merchant_app/models/order.dart';
import 'package:merchant_app/widgets/merchant_content_shell.dart';
import 'package:merchant_app/widgets/merchant_primary_button.dart';
import 'package:merchant_app/widgets/merchant_status_badge.dart';

class OrderListPage extends ConsumerStatefulWidget {
  const OrderListPage({super.key});

  @override
  ConsumerState<OrderListPage> createState() => _OrderListPageState();
}

class _OrderListPageState extends ConsumerState<OrderListPage> {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (ref.read(workingStatusProvider).isOnline) {
        ref.read(orderProvider.notifier).fetchOrders();
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    ref.listen(workingStatusProvider.select((state) => state.isOnline), (
      previous,
      next,
    ) {
      if (next) {
        ref.read(orderProvider.notifier).fetchOrders();
      } else if (previous == true) {
        ref.read(orderProvider.notifier).clearOrders();
      }
    });

    final authState = ref.watch(authProvider);
    final orderState = ref.watch(orderProvider);
    final workingStatusState = ref.watch(workingStatusProvider);
    final isWorking = workingStatusState.isOnline;
    final isStatusBusy =
        workingStatusState.isLoading || workingStatusState.isUpdating;
    final isInitialStatusSync =
        workingStatusState.isLoading && !workingStatusState.hasConfirmedState;
    final hasNetwork = ref.watch(connectivityProvider).value ?? true;
    final isWsConnected = ref.watch(wsStatusProvider);
    final isAuthenticated = authState.isAuthenticated;
    final merchantName = authState.merchantName;
    final canReceiveOrders = isAuthenticated && isWorking;
    final displayTitle =
        (merchantName != null && merchantName.trim().isNotEmpty)
        ? merchantName.trim()
        : (isAuthenticated ? '商户工作台' : '未绑定商户');

    final showWsWarning = canReceiveOrders && hasNetwork && !isWsConnected;

    return DefaultTabController(
      length: 3,
      child: Scaffold(
        appBar: AppBar(
          title: Text(
            isInitialStatusSync
                ? '$displayTitle (状态同步中)'
                : canReceiveOrders
                ? '$displayTitle (在线营业)'
                : isAuthenticated
                ? (workingStatusState.error != null &&
                          !workingStatusState.hasConfirmedState
                      ? '$displayTitle (状态异常)'
                      : '$displayTitle (离线打烊)')
                : '$displayTitle (待绑定)',
          ),
          backgroundColor: canReceiveOrders
              ? Theme.of(context).colorScheme.surface
              : Theme.of(context).colorScheme.surfaceContainerLow,
          foregroundColor: Theme.of(context).colorScheme.onSurface,
          bottom: const TabBar(
            tabs: [
              Tab(text: '待接单'),
              Tab(text: '进行中'),
              Tab(text: '已完成'),
            ],
          ),
          actions: [
            Switch(
              value: canReceiveOrders,
              onChanged: isAuthenticated && !isStatusBusy
                  ? (val) async {
                      final result = await ref
                          .read(workingStatusProvider.notifier)
                          .setStatus(val);
                      final latestStatus = ref.read(workingStatusProvider);
                      if (context.mounted && latestStatus.error != null) {
                        ScaffoldMessenger.of(context).showSnackBar(
                          SnackBar(content: Text(latestStatus.error!)),
                        );
                      }
                      if (result) {
                        ref.read(orderProvider.notifier).fetchOrders();
                      }
                    }
                  : null,
              activeThumbColor: AppColors.surfaceLowest,
              activeTrackColor: AppColors.primaryContainer,
            ),
            IconButton(
              icon: const Icon(Icons.refresh),
              onPressed: canReceiveOrders
                  ? () => ref.read(orderProvider.notifier).fetchOrders()
                  : null,
            ),
          ],
        ),
        drawer: Drawer(
          child: ListView(
            padding: EdgeInsets.zero,
            children: [
              DrawerHeader(
                decoration: BoxDecoration(
                  color: canReceiveOrders
                      ? AppColors.primary
                      : Theme.of(context).colorScheme.surfaceContainerLow,
                ),
                child: Column(
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    const CircleAvatar(
                      radius: 30,
                      backgroundColor: Colors.white,
                      child: Icon(
                        Icons.storefront_rounded,
                        color: AppColors.primary,
                        size: 30,
                      ),
                    ),
                    const SizedBox(height: 10),
                    Text(
                      displayTitle,
                      style: TextStyle(
                        color: canReceiveOrders
                            ? Colors.white
                            : Theme.of(context).colorScheme.onSurface,
                        fontSize: 18,
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                    const SizedBox(height: 4),
                    Text(
                      isAuthenticated ? '商户工作台' : '请先绑定商户后开始接单',
                      style: TextStyle(
                        color: canReceiveOrders
                            ? Colors.white.withValues(alpha: 0.86)
                            : Theme.of(context).colorScheme.onSurfaceVariant,
                        fontSize: 12,
                      ),
                    ),
                  ],
                ),
              ),
              ListTile(
                leading: const Icon(Icons.list),
                title: const Text('订单管理'),
                selected: true,
                onTap: () => Navigator.pop(context),
              ),
              ListTile(
                leading: const Icon(Icons.table_restaurant),
                title: const Text('桌台管理'),
                onTap: () {
                  Navigator.pop(context);
                  context.push('/tables');
                },
              ),
              ListTile(
                leading: const Icon(Icons.settings),
                title: const Text('系统设置'),
                onTap: () {
                  Navigator.pop(context);
                  context.push('/settings');
                },
              ),
              ListTile(
                leading: Icon(isAuthenticated ? Icons.logout : Icons.login),
                title: Text(isAuthenticated ? '退出登录' : '绑定商户'),
                onTap: () async {
                  Navigator.pop(context);
                  if (!isAuthenticated) {
                    context.go('/login');
                    return;
                  }

                  ref.read(workingStatusProvider.notifier).resetLocal();
                  ref.read(orderProvider.notifier).clearOrders();
                  await ref.read(authProvider.notifier).logout();
                },
              ),
            ],
          ),
        ),
        body: !isAuthenticated
            ? MerchantContentShell(
                child: Center(
                  child: Container(
                    padding: const EdgeInsets.all(AppSpacing.xxl),
                    decoration: BoxDecoration(
                      color: Theme.of(
                        context,
                      ).colorScheme.surfaceContainerLowest,
                      borderRadius: BorderRadius.circular(AppRadius.xl),
                    ),
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Container(
                          width: 88,
                          height: 88,
                          decoration: BoxDecoration(
                            color: AppColors.surfaceLow,
                            borderRadius: BorderRadius.circular(AppRadius.xl),
                          ),
                          child: Icon(
                            Icons.lock_person_outlined,
                            size: 44,
                            color: Theme.of(
                              context,
                            ).colorScheme.onSurfaceVariant,
                          ),
                        ),
                        const SizedBox(height: AppSpacing.xl),
                        const Text(
                          '尚未绑定商户',
                          style: TextStyle(
                            fontSize: 20,
                            fontWeight: FontWeight.w700,
                          ),
                        ),
                        const SizedBox(height: AppSpacing.sm),
                        Text(
                          '绑定商户后可开始接单、上线营业和同步订单。当前可以先查看协议、保活指引、通知设置和蓝牙打印机设置。',
                          textAlign: TextAlign.center,
                          style: TextStyle(
                            color: Theme.of(
                              context,
                            ).colorScheme.onSurfaceVariant,
                            height: 1.5,
                          ),
                        ),
                        const SizedBox(height: AppSpacing.xl),
                        MerchantPrimaryButton(
                          label: '立即绑定商户',
                          onPressed: () => context.go('/login'),
                        ),
                      ],
                    ),
                  ),
                ),
              )
            : isInitialStatusSync
            ? const MerchantContentShell(
                child: Center(
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      CircularProgressIndicator(),
                      SizedBox(height: AppSpacing.lg),
                      Text('正在同步营业状态...'),
                    ],
                  ),
                ),
              )
            : workingStatusState.error != null &&
                  !workingStatusState.hasConfirmedState
            ? MerchantContentShell(
                child: Center(
                  child: Container(
                    padding: const EdgeInsets.all(AppSpacing.xxl),
                    decoration: BoxDecoration(
                      color: Theme.of(
                        context,
                      ).colorScheme.surfaceContainerLowest,
                      borderRadius: BorderRadius.circular(AppRadius.xl),
                    ),
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Container(
                          width: 88,
                          height: 88,
                          decoration: BoxDecoration(
                            color: AppColors.warningSoft,
                            borderRadius: BorderRadius.circular(AppRadius.xl),
                          ),
                          child: Icon(
                            Icons.sync_problem_rounded,
                            size: 44,
                            color: Theme.of(
                              context,
                            ).colorScheme.onSurfaceVariant,
                          ),
                        ),
                        const SizedBox(height: AppSpacing.xl),
                        const Text(
                          '营业状态同步失败',
                          style: TextStyle(
                            fontSize: 20,
                            fontWeight: FontWeight.w700,
                          ),
                        ),
                        const SizedBox(height: AppSpacing.sm),
                        Text(
                          workingStatusState.error!,
                          textAlign: TextAlign.center,
                          style: TextStyle(
                            color: Theme.of(
                              context,
                            ).colorScheme.onSurfaceVariant,
                            height: 1.5,
                          ),
                        ),
                        const SizedBox(height: AppSpacing.xl),
                        MerchantPrimaryButton(
                          label: '重新同步',
                          isLoading: workingStatusState.isLoading,
                          onPressed: workingStatusState.isLoading
                              ? null
                              : () => ref
                                    .read(workingStatusProvider.notifier)
                                    .syncFromBackend(),
                        ),
                      ],
                    ),
                  ),
                ),
              )
            : !isWorking
            ? MerchantContentShell(
                child: Center(
                  child: Container(
                    padding: const EdgeInsets.all(AppSpacing.xxl),
                    decoration: BoxDecoration(
                      color: Theme.of(
                        context,
                      ).colorScheme.surfaceContainerLowest,
                      borderRadius: BorderRadius.circular(AppRadius.xl),
                    ),
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Container(
                          width: 88,
                          height: 88,
                          decoration: BoxDecoration(
                            color: AppColors.surfaceLow,
                            borderRadius: BorderRadius.circular(AppRadius.xl),
                          ),
                          child: Icon(
                            Icons.store_mall_directory_outlined,
                            size: 44,
                            color: Theme.of(
                              context,
                            ).colorScheme.onSurfaceVariant,
                          ),
                        ),
                        const SizedBox(height: AppSpacing.xl),
                        const Text(
                          '当前处于打烊状态',
                          style: TextStyle(
                            fontSize: 20,
                            fontWeight: FontWeight.w700,
                          ),
                        ),
                        const SizedBox(height: AppSpacing.sm),
                        Text(
                          '上线营业后才能接收新订单和断线补单。',
                          textAlign: TextAlign.center,
                          style: TextStyle(
                            color: Theme.of(
                              context,
                            ).colorScheme.onSurfaceVariant,
                            height: 1.5,
                          ),
                        ),
                        const SizedBox(height: AppSpacing.xl),
                        MerchantPrimaryButton(
                          label: '立即上线营业',
                          isLoading: workingStatusState.isUpdating,
                          onPressed: workingStatusState.isUpdating
                              ? null
                              : () async {
                                  final result = await ref
                                      .read(workingStatusProvider.notifier)
                                      .setStatus(true);
                                  final latestStatus = ref.read(
                                    workingStatusProvider,
                                  );
                                  if (context.mounted &&
                                      latestStatus.error != null) {
                                    ScaffoldMessenger.of(context).showSnackBar(
                                      SnackBar(
                                        content: Text(latestStatus.error!),
                                      ),
                                    );
                                  }
                                  if (result) {
                                    ref
                                        .read(orderProvider.notifier)
                                        .fetchOrders();
                                  }
                                },
                        ),
                      ],
                    ),
                  ),
                ),
              )
            : Column(
                children: [
                  if (!hasNetwork || showWsWarning)
                    Container(
                      color: !hasNetwork
                          ? AppColors.dangerSoft
                          : AppColors.warningSoft,
                      padding: const EdgeInsets.all(8),
                      child: Row(
                        mainAxisAlignment: MainAxisAlignment.center,
                        children: [
                          Icon(
                            Icons.warning_amber_rounded,
                            color: !hasNetwork
                                ? AppColors.tertiary
                                : AppColors.secondary,
                          ),
                          const SizedBox(width: 8),
                          Text(
                            !hasNetwork ? '无网络连接，可能漏单请检查网络' : '服务器已断开，正在重连...',
                            style: TextStyle(
                              color: !hasNetwork
                                  ? AppColors.tertiary
                                  : AppColors.onSurface,
                              fontWeight: FontWeight.w700,
                            ),
                          ),
                        ],
                      ),
                    ),
                  Expanded(
                    child: orderState.isLoading && orderState.orders.isEmpty
                        ? const Center(child: CircularProgressIndicator())
                        : TabBarView(
                            children: [
                              _OrderList(
                                orders: orderState.orders
                                    .where((o) => o.isAwaitingAcceptance)
                                    .toList(),
                                canOpenDetail: isAuthenticated,
                              ),
                              _OrderList(
                                orders: orderState.orders
                                    .where(
                                      (o) =>
                                          !o.isAwaitingAcceptance &&
                                          o.status != OrderStatus.completed &&
                                          o.status != OrderStatus.cancelled,
                                    )
                                    .toList(),
                                canOpenDetail: isAuthenticated,
                              ),
                              _OrderList(
                                orders: orderState.orders
                                    .where(
                                      (o) =>
                                          o.status == OrderStatus.completed ||
                                          o.status == OrderStatus.cancelled,
                                    )
                                    .toList(),
                                canOpenDetail: isAuthenticated,
                              ),
                            ],
                          ),
                  ),
                ],
              ),
      ),
    );
  }
}

class _OrderList extends ConsumerWidget {
  final List<OrderModel> orders;
  final bool canOpenDetail;

  const _OrderList({required this.orders, required this.canOpenDetail});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    if (orders.isEmpty) {
      return Center(
        child: Text(
          '暂无订单',
          style: TextStyle(
            color: Theme.of(context).colorScheme.onSurfaceVariant,
          ),
        ),
      );
    }

    return ListView.builder(
      padding: const EdgeInsets.all(AppSpacing.sm),
      itemCount: orders.length,
      itemBuilder: (context, index) {
        final order = orders[index];
        final isProcessing = ref.watch(
          orderProvider.select(
            (state) => state.actionInFlightOrderIds.contains(order.id),
          ),
        );
        return Card(
          margin: const EdgeInsets.symmetric(
            vertical: AppSpacing.sm,
            horizontal: AppSpacing.xs,
          ),
          child: InkWell(
            onTap: canOpenDetail
                ? () => context.push('/order-detail', extra: order)
                : null,
            borderRadius: BorderRadius.circular(AppRadius.xl),
            child: Padding(
              padding: const EdgeInsets.all(AppSpacing.xl),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(
                              '订单 ${order.orderNum}',
                              style: const TextStyle(
                                fontSize: 17,
                                fontWeight: FontWeight.w700,
                              ),
                            ),
                            const SizedBox(height: AppSpacing.xs),
                            Text(
                              order.formattedDate,
                              style: const TextStyle(
                                color: AppColors.onSurfaceVariant,
                                fontSize: 12,
                              ),
                            ),
                          ],
                        ),
                      ),
                      MerchantStatusBadge(
                        label: order.status.label,
                        tone: _statusToneFor(order.status),
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
                    child: Column(
                      children: [
                        if (!order.hasReliableItems)
                          const _OrderItemsDegradedText()
                        else if (order.items.isEmpty)
                          const _OrderItemsEmptyText()
                        else
                          ...order.items.map(
                            (item) => _OrderItemLine(item: item),
                          ),
                        if ((order.note ?? '').trim().isNotEmpty) ...[
                          const SizedBox(height: AppSpacing.md),
                          Align(
                            alignment: Alignment.centerLeft,
                            child: Container(
                              padding: const EdgeInsets.symmetric(
                                horizontal: AppSpacing.md,
                                vertical: AppSpacing.sm,
                              ),
                              decoration: BoxDecoration(
                                color: Colors.white,
                                borderRadius: BorderRadius.circular(
                                  AppRadius.md,
                                ),
                              ),
                              child: Text(
                                '备注：${order.note!.trim()}',
                                style: const TextStyle(
                                  color: AppColors.onSurfaceVariant,
                                  height: 1.45,
                                ),
                              ),
                            ),
                          ),
                        ],
                      ],
                    ),
                  ),
                  const SizedBox(height: AppSpacing.lg),
                  Row(
                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    crossAxisAlignment: CrossAxisAlignment.end,
                    children: [
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(
                              order.hasReliableItems
                                  ? '${order.items.length} 件商品'
                                  : '明细同步中',
                              style: const TextStyle(
                                color: AppColors.onSurfaceVariant,
                                fontSize: 12,
                              ),
                            ),
                            if ((order.userName ?? '').trim().isNotEmpty)
                              Text(
                                order.userName!.trim(),
                                maxLines: 1,
                                overflow: TextOverflow.ellipsis,
                                style: const TextStyle(
                                  color: AppColors.onSurfaceVariant,
                                  fontSize: 12,
                                ),
                              ),
                            if (order.deliveryFeeDisplayAmount > 0)
                              Text(
                                '代取费另列 ¥${order.deliveryFeeDisplayAmount.toStringAsFixed(2)}',
                                maxLines: 1,
                                overflow: TextOverflow.ellipsis,
                                style: const TextStyle(
                                  color: AppColors.onSurfaceVariant,
                                  fontSize: 12,
                                ),
                              ),
                          ],
                        ),
                      ),
                      const SizedBox(width: AppSpacing.md),
                      Flexible(
                        child: Text(
                          '餐费: ¥${order.merchantFoodDisplayAmount.toStringAsFixed(2)}',
                          maxLines: 2,
                          overflow: TextOverflow.ellipsis,
                          textAlign: TextAlign.right,
                          style: const TextStyle(
                            fontSize: 18,
                            fontWeight: FontWeight.w700,
                            color: AppColors.secondary,
                          ),
                        ),
                      ),
                    ],
                  ),
                  if (order.isAwaitingAcceptance)
                    Padding(
                      padding: const EdgeInsets.only(top: AppSpacing.lg),
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
                                  final printerState = ref.read(
                                    printerProvider,
                                  );
                                  final merchantName =
                                      ref.read(authProvider).merchantName ??
                                      '商户工作台';
                                  if (notificationSettings
                                          .autoPrintAfterAcceptEnabled &&
                                      printerState.connectedDevice != null) {
                                    await ref
                                        .read(printerProvider.notifier)
                                        .printAcceptedOrder(
                                          order,
                                          shopName: merchantName,
                                        );
                                  }
                                }
                              },
                      ),
                    ),
                ],
              ),
            ),
          ),
        );
      },
    );
  }
}

class _OrderItemLine extends StatelessWidget {
  final OrderItem item;

  const _OrderItemLine({required this.item});

  @override
  Widget build(BuildContext context) {
    final specsText = item.specsText.trim();
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: AppSpacing.xs),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  '${item.name} x${item.quantity}',
                  style: const TextStyle(height: 1.4),
                ),
                if (specsText.isNotEmpty)
                  Text(
                    specsText,
                    style: const TextStyle(
                      color: AppColors.onSurfaceVariant,
                      fontSize: 12,
                      height: 1.35,
                    ),
                  ),
              ],
            ),
          ),
          const SizedBox(width: AppSpacing.md),
          Text('¥${item.lineTotal.toStringAsFixed(2)}'),
        ],
      ),
    );
  }
}

class _OrderItemsDegradedText extends StatelessWidget {
  const _OrderItemsDegradedText();

  @override
  Widget build(BuildContext context) {
    return const Align(
      alignment: Alignment.centerLeft,
      child: Text(
        '订单明细正在自动同步，请稍候。',
        style: TextStyle(color: AppColors.onSurfaceVariant, height: 1.45),
      ),
    );
  }
}

class _OrderItemsEmptyText extends StatelessWidget {
  const _OrderItemsEmptyText();

  @override
  Widget build(BuildContext context) {
    return const Align(
      alignment: Alignment.centerLeft,
      child: Text(
        '暂无商品明细',
        style: TextStyle(color: AppColors.onSurfaceVariant, height: 1.45),
      ),
    );
  }
}

MerchantStatusTone _statusToneFor(OrderStatus status) {
  switch (status) {
    case OrderStatus.pending:
      return MerchantStatusTone.neutral;
    case OrderStatus.cancelled:
      return MerchantStatusTone.danger;
    case OrderStatus.completed:
      return MerchantStatusTone.neutral;
    case OrderStatus.paid:
      return MerchantStatusTone.warning;
    case OrderStatus.preparing:
    case OrderStatus.accepted:
    case OrderStatus.ready:
    case OrderStatus.courierAccepted:
    case OrderStatus.picked:
    case OrderStatus.delivering:
    case OrderStatus.riderDelivered:
    case OrderStatus.userDelivered:
      return MerchantStatusTone.positive;
    case OrderStatus.unknown:
      return MerchantStatusTone.neutral;
  }
}
