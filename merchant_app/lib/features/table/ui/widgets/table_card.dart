import 'package:flutter/material.dart';
import 'package:merchant_app/config/theme.dart';
import 'package:merchant_app/features/table/models/table_model.dart';

class TableCard extends StatelessWidget {
  final TableModel table;
  final VoidCallback onTap;

  const TableCard({super.key, required this.table, required this.onTap});

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

    final hasImage = table.imageUrl != null && table.imageUrl!.isNotEmpty;

    return Card(
      elevation: 0,
      margin: EdgeInsets.zero,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(AppRadius.lg),
        side: BorderSide(color: borderColor, width: 1),
      ),
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: onTap,
        child: Stack(
          fit: StackFit.expand,
          children: [
            // Background Image (if any)
            if (hasImage)
              Positioned.fill(
                child: Opacity(
                  opacity: 0.3,
                  child: Image.network(
                    table.imageUrl!,
                    fit: BoxFit.cover,
                    errorBuilder: (context, error, stackTrace) =>
                        const SizedBox(),
                  ),
                ),
              )
            else
              Positioned.fill(child: Container(color: bgColor)),

            // Content
            Padding(
              padding: const EdgeInsets.all(AppSpacing.md),
              child: Column(
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  Text(
                    table.tableNo,
                    style: TextStyle(
                      fontSize: 20,
                      fontWeight: FontWeight.w700,
                      color: isOccupied
                          ? AppColors.warning
                          : AppColors.onSurface,
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
                  if (table.minimumSpend != null)
                    Text(
                      '低消 ¥${(table.minimumSpend! / 100).toStringAsFixed(0)}',
                      style: TextStyle(fontSize: 10, color: textColor),
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
                        Flexible(
                          child: Text(
                            table.currentReservation!.reservationTime,
                            style: TextStyle(
                              fontSize: 10,
                              color: textColor,
                              fontWeight: FontWeight.w600,
                            ),
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                          ),
                        ),
                      ],
                    ),
                  ],
                ],
              ),
            ),

            // Tags indicator (top right)
            if (table.tags.isNotEmpty)
              Positioned(
                top: 8,
                right: 8,
                child: Row(
                  mainAxisSize: MainAxisSize.min,
                  children: table.tags
                      .take(2)
                      .map(
                        (tag) => Container(
                          margin: const EdgeInsets.only(left: 2),
                          padding: const EdgeInsets.symmetric(
                            horizontal: 4,
                            vertical: 2,
                          ),
                          decoration: BoxDecoration(
                            color: AppColors.primary.withValues(alpha: 0.1),
                            borderRadius: BorderRadius.circular(4.0),
                          ),
                          child: Text(
                            tag.name,
                            style: const TextStyle(
                              fontSize: 8,
                              color: AppColors.primary,
                              fontWeight: FontWeight.w600,
                            ),
                          ),
                        ),
                      )
                      .toList(),
                ),
              ),
          ],
        ),
      ),
    );
  }
}
