import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  test('table image upload widgets use backend table media category', () {
    final backendTableHandler = File(
      '../locallife/api/table.go',
    ).readAsStringSync();
    final backendMediaPolicy = File(
      '../locallife/media/policy.go',
    ).readAsStringSync();

    expect(
      backendTableHandler,
      contains('string(media.CategoryTableImage)'),
      reason: 'Backend table image binding must remain tied to media.CategoryTableImage.',
    );
    expect(
      RegExp(
        r'CategoryTableImage\s+Category\s*=\s*"table"',
      ).hasMatch(backendMediaPolicy),
      isTrue,
      reason: 'Backend media.CategoryTableImage must resolve to media_category=table.',
    );

    const tableImagePickerPaths = <String>[
      'lib/features/table/ui/widgets/table_config_sheet.dart',
      'lib/features/table/ui/widgets/table_image_gallery.dart',
    ];

    for (final path in tableImagePickerPaths) {
      final source = File(path).readAsStringSync();
      final categories = RegExp(
        r"mediaCategory:\s*'([^']+)'",
      ).allMatches(source).map((match) => match.group(1)).toList();

      expect(
        categories,
        isNotEmpty,
        reason: '$path must declare the table image upload media category.',
      );
      expect(
        categories,
        everyElement('table'),
        reason: '$path must upload table images with media_category=table.',
      );
      expect(
        source,
        isNot(contains('table_cover')),
        reason: '$path must not use the stale table_cover category.',
      );
    }
  });
}
