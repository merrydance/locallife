import 'package:flutter/material.dart';
import 'package:merchant_app/config/theme.dart';

class MerchantContentShell extends StatelessWidget {
  const MerchantContentShell({
    required this.child,
    super.key,
    this.maxWidth = 420,
    this.padding = const EdgeInsets.symmetric(
      horizontal: AppSpacing.viewportHorizontal,
      vertical: AppSpacing.viewportVertical,
    ),
  });

  final Widget child;
  final double maxWidth;
  final EdgeInsetsGeometry padding;

  @override
  Widget build(BuildContext context) {
    return SafeArea(
      child: Align(
        alignment: Alignment.topCenter,
        child: ConstrainedBox(
          constraints: BoxConstraints(maxWidth: maxWidth),
          child: Padding(padding: padding, child: child),
        ),
      ),
    );
  }
}
