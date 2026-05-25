import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:merchant_app/config/theme.dart';
import 'package:merchant_app/widgets/merchant_content_shell.dart';

void main() {
  testWidgets('MerchantContentShell uses compact shared viewport padding', (
    tester,
  ) async {
    await tester.pumpWidget(
      const MaterialApp(
        home: Scaffold(
          body: MerchantContentShell(child: SizedBox(key: Key('content'))),
        ),
      ),
    );

    final padding = tester.widget<Padding>(
      find
          .ancestor(
            of: find.byKey(const Key('content')),
            matching: find.byType(Padding),
          )
          .first,
    );

    expect(
      padding.padding,
      const EdgeInsets.symmetric(
        horizontal: AppSpacing.viewportHorizontal,
        vertical: AppSpacing.viewportVertical,
      ),
    );
  });
}
