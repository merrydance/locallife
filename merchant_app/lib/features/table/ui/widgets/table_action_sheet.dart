import 'package:flutter/material.dart';
import 'package:merchant_app/config/theme.dart';
import 'package:merchant_app/features/table/models/table_model.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:merchant_app/features/table/providers/table_provider.dart';
import 'package:merchant_app/features/table/ui/widgets/table_config_sheet.dart';
import 'package:merchant_app/widgets/merchant_primary_button.dart';

class TableActionSheet extends ConsumerWidget {
  final TableModel table;
  final Future<bool> Function(String status) onUpdateStatus;

  const TableActionSheet({
    super.key,
    required this.table,
    required this.onUpdateStatus,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final isAvailable = table.status == TableStatus.available;
    final tableState = ref.watch(tableProvider);
    final isLoading = tableState.actionInFlightTableIds.contains(table.id);

    return Container(
      padding: const EdgeInsets.all(AppSpacing.xl),
      decoration: const BoxDecoration(
        color: Colors.white,
        borderRadius: BorderRadius.vertical(top: Radius.circular(AppRadius.xxl)),
      ),
      child: SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      '桌台 ${table.tableNo}',
                      style: const TextStyle(
                        fontSize: 20,
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                    const SizedBox(height: 4),
                    Text(
                      '${table.capacity}座 · ${table.tableType == TableType.room ? '包间' : '大厅散座'}',
                      style: const TextStyle(
                        color: AppColors.onSurfaceVariant,
                        fontSize: 14,
                      ),
                    ),
                  ],
                ),
                Container(
                  padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
                  decoration: BoxDecoration(
                    color: isAvailable ? AppColors.positiveSoft : AppColors.warningSoft,
                    borderRadius: BorderRadius.circular(100),
                  ),
                  child: Text(
                    isAvailable ? '空闲中' : (table.status == TableStatus.occupied ? '就餐中' : '已停用'),
                    style: TextStyle(
                      color: isAvailable ? AppColors.positive : AppColors.warning,
                      fontWeight: FontWeight.w600,
                      fontSize: 12,
                    ),
                  ),
                ),
              ],
            ),
            const SizedBox(height: AppSpacing.xxl),
            if (isAvailable) ...[
              MerchantPrimaryButton(
                label: '开台 (置为占用)',
                expand: true,
                isLoading: isLoading,
                onPressed: () async {
                  final success = await onUpdateStatus('occupied');
                  if (success && context.mounted) {
                    Navigator.pop(context);
                  } else if (context.mounted && tableState.error != null) {
                    ScaffoldMessenger.of(context).showSnackBar(
                      SnackBar(content: Text(tableState.error!)),
                    );
                  }
                },
              ),
            ] else if (table.status == TableStatus.occupied) ...[
              MerchantPrimaryButton(
                label: '清台 (结束就餐)',
                expand: true,
                isLoading: isLoading,
                onPressed: () async {
                  final success = await onUpdateStatus('available');
                  if (success && context.mounted) {
                    Navigator.pop(context);
                  } else if (context.mounted && tableState.error != null) {
                    ScaffoldMessenger.of(context).showSnackBar(
                      SnackBar(content: Text(tableState.error!)),
                    );
                  }
                },
              ),
            ],
            const SizedBox(height: AppSpacing.md),
            SizedBox(
              width: double.infinity,
              height: 48,
              child: TextButton(
                onPressed: () {
                  Navigator.pop(context);
                  showModalBottomSheet(
                    context: context,
                    isScrollControlled: true,
                    backgroundColor: Colors.transparent,
                    builder: (context) => TableConfigSheet(existingTable: table),
                  );
                },
                style: TextButton.styleFrom(
                  foregroundColor: AppColors.onSurfaceVariant,
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(AppRadius.lg),
                  ),
                ),
                child: const Text('桌台设置'),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
