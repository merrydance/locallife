import 'package:flutter_test/flutter_test.dart';
import 'package:merchant_app/features/table/models/table_model.dart';

void main() {
  test('parsing', () {
    final jsonMap = {
      "id": 1,
      "merchant_id": 1,
      "table_no": "A1",
      "table_type": "table",
      "capacity": 4,
      "status": "available",
      "created_at": "2023-01-01T00:00:00Z",
    };
    final model = TableModel.fromJson(jsonMap);
    expect(model.tableNo, 'A1');
  });
}
