import 'package:flutter/material.dart';

class MerchantPrimaryButton extends StatelessWidget {
  const MerchantPrimaryButton({
    required this.label,
    required this.onPressed,
    super.key,
    this.expand = false,
    this.maxWidth = 360,
    this.icon,
    this.isLoading = false,
  });

  final String label;
  final VoidCallback? onPressed;
  final bool expand;
  final double maxWidth;
  final Widget? icon;
  final bool isLoading;

  @override
  Widget build(BuildContext context) {
    final labelWidget = isLoading
        ? const SizedBox(
            width: 18,
            height: 18,
            child: CircularProgressIndicator(strokeWidth: 2),
          )
        : Text(label);

    final button = icon == null || isLoading
        ? ElevatedButton(
            onPressed: isLoading ? null : onPressed,
            child: labelWidget,
          )
        : ElevatedButton.icon(
            onPressed: onPressed,
            icon: icon!,
            label: labelWidget,
          );

    if (expand) {
      return SizedBox(width: double.infinity, child: button);
    }

    return Align(
      alignment: Alignment.center,
      child: ConstrainedBox(
        constraints: BoxConstraints(maxWidth: maxWidth),
        child: SizedBox(width: double.infinity, child: button),
      ),
    );
  }
}