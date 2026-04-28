import 'package:flutter/material.dart';
import 'package:merchant_app/config/theme.dart';
import 'package:merchant_app/features/table/models/table_model.dart';

class TableCard extends StatelessWidget {
  final TableModel table;
  final VoidCallback onTap;

  const TableCard({
    super.key,
    required this.table,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final bool isAvailable = table.status == TableStatus.available;
    final bool isOccupied = table.status == TableStatus.occupied;

    Color bgColor = AppColors.surfaceLow;
    Color textColor = AppColors.onSurfaceVariant;
    Color borderColor = Colors.transparent;

    if (isAvailable) {
      bgColor = AppColors.positiveSoft;
      textColor = AppColors.positive;
      borderColor = AppColors.positive.withValues(alpha: 0.3);
    } else if (isOccupied) {
      bgColor = AppColors.warningSoft;
      textColor = AppColors.warning;
      borderColor = AppColors.warning.withValues(alpha: 0.3);
    }

    return Card(
      elevation: 0,
      margin: EdgeInsets.zero,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(AppRadius.lg),
        side: BorderSide(color: borderColor, width: 1),
      ),
      color: bgColor,
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(AppRadius.lg),
        child: Padding(
          padding: const EdgeInsets.all(AppSpacing.md),
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              Text(
                table.tableNo,
                style: TextStyle(
                  fontSize: 20,
                  fontWeight: FontWeight.w700,
                  color: isOccupied ? AppColors.warning : AppColors.onSurface,
                ),
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
              ),
              const SizedBox(height: AppSpacing.xs),
              Text(
                '${table.capacity}座',
                style: TextStyle(
                  fontSize: 12,
                  color: textColor,
                  fontWeight: FontWeight.w500,
                ),
              ),
              if (table.currentReservation != null) ...[
                const Spacer(),
                Row(
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    Icon(
                      Icons.access_time_filled,
                      size: 12,
                      color: textColor,
                    ),
                    const SizedBox(width: 4),
                    Text(
                      table.currentReservation!.time,
                      style: TextStyle(
                        fontSize: 10,
                        color: textColor,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                  ],
                ),
              ],
            ],
          ),
        ),
      ),
    );
  }
}
