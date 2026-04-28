import 'package:flutter/material.dart';
import 'package:merchant_app/config/theme.dart';

enum MerchantStatusTone { neutral, positive, warning, danger }

class MerchantStatusBadge extends StatelessWidget {
  const MerchantStatusBadge({
    required this.label,
    required this.tone,
    super.key,
  });

  final String label;
  final MerchantStatusTone tone;

  @override
  Widget build(BuildContext context) {
    final palette = switch (tone) {
      MerchantStatusTone.positive => (
          background: const Color(0xFFE5F5EA),
          foreground: AppColors.primary,
        ),
      MerchantStatusTone.warning => (
          background: AppColors.warningSoft,
          foreground: AppColors.secondary,
        ),
      MerchantStatusTone.danger => (
          background: AppColors.dangerSoft,
          foreground: AppColors.tertiary,
        ),
      MerchantStatusTone.neutral => (
          background: AppColors.surfaceLow,
          foreground: AppColors.onSurfaceVariant,
        ),
    };

    return Container(
      padding: const EdgeInsets.symmetric(
        horizontal: AppSpacing.md,
        vertical: AppSpacing.sm,
      ),
      decoration: BoxDecoration(
        color: palette.background,
        borderRadius: BorderRadius.circular(AppRadius.pill),
      ),
      child: Text(
        label,
        style: TextStyle(
          color: palette.foreground,
          fontSize: 12,
          fontWeight: FontWeight.w700,
        ),
      ),
    );
  }
}