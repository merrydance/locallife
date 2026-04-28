import 'package:flutter/material.dart';
import 'package:merchant_app/config/theme.dart';

class MerchantSecondaryButton extends StatelessWidget {
  const MerchantSecondaryButton({
    required this.label,
    required this.onPressed,
    super.key,
    this.expand = false,
    this.icon,
  });

  final String label;
  final VoidCallback? onPressed;
  final bool expand;
  final Widget? icon;

  @override
  Widget build(BuildContext context) {
    final button = icon == null
        ? OutlinedButton(
            onPressed: onPressed,
            child: Text(label),
          )
        : OutlinedButton.icon(
            onPressed: onPressed,
            icon: icon!,
            label: Text(label),
          );

    final themed = Theme(
      data: Theme.of(context).copyWith(
        outlinedButtonTheme: OutlinedButtonThemeData(
          style: OutlinedButton.styleFrom(
            minimumSize: const Size(0, 56),
            padding: const EdgeInsets.symmetric(
              horizontal: AppSpacing.xl,
              vertical: AppSpacing.lg,
            ),
            foregroundColor: AppColors.onSurface,
            side: const BorderSide(color: AppColors.outlineVariant),
            shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(AppRadius.lg),
            ),
          ),
        ),
      ),
      child: button,
    );

    if (expand) {
      return SizedBox(width: double.infinity, child: themed);
    }

    return Align(
      alignment: Alignment.center,
      child: SizedBox(width: double.infinity, child: themed),
    );
  }
}