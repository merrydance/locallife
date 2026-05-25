import 'package:flutter/material.dart';
import 'package:merchant_app/config/theme.dart';
import 'package:merchant_app/features/table/models/table_model.dart';

class TableFilterBar extends StatelessWidget {
  final TableStatus? selectedStatus;
  final ValueChanged<TableStatus?> onStatusChanged;

  const TableFilterBar({
    super.key,
    required this.selectedStatus,
    required this.onStatusChanged,
  });

  @override
  Widget build(BuildContext context) {
    return SingleChildScrollView(
      scrollDirection: Axis.horizontal,
      padding: const EdgeInsets.symmetric(
        horizontal: AppSpacing.lg,
        vertical: AppSpacing.md,
      ),
      child: Row(
        children: [
          _buildFilterChip(null, '全部'),
          const SizedBox(width: AppSpacing.sm),
          _buildFilterChip(TableStatus.available, TableStatus.available.label),
          const SizedBox(width: AppSpacing.sm),
          _buildFilterChip(TableStatus.occupied, TableStatus.occupied.label),
          const SizedBox(width: AppSpacing.sm),
          _buildFilterChip(TableStatus.disabled, TableStatus.disabled.label),
        ],
      ),
    );
  }

  Widget _buildFilterChip(TableStatus? status, String label) {
    final isSelected = selectedStatus == status;
    return ChoiceChip(
      label: Text(label),
      selected: isSelected,
      onSelected: (_) => onStatusChanged(status),
      selectedColor: AppColors.primary,
      backgroundColor: AppColors.surfaceLow,
      labelStyle: TextStyle(
        color: isSelected ? Colors.white : AppColors.onSurfaceVariant,
        fontWeight: isSelected ? FontWeight.w600 : FontWeight.normal,
      ),
      side: BorderSide.none,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(AppRadius.pill),
      ),
    );
  }
}
