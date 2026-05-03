import 'package:flutter_test/flutter_test.dart';
import 'package:merchant_app/core/network/api_client.dart';

void main() {
  test('extractApiResponseData unwraps unified response envelope', () {
    final data = extractApiResponseData(<String, dynamic>{
      'code': 0,
      'message': 'ok',
      'data': <String, dynamic>{
        'access_token': 'access-token',
        'refresh_token': 'refresh-token',
      },
    });

    expect(data, <String, dynamic>{
      'access_token': 'access-token',
      'refresh_token': 'refresh-token',
    });
  });

  test('extractApiResponseData preserves legacy unwrapped token response', () {
    final data = extractApiResponseData(<String, dynamic>{
      'access_token': 'access-token',
      'refresh_token': 'refresh-token',
    });

    expect(data, <String, dynamic>{
      'access_token': 'access-token',
      'refresh_token': 'refresh-token',
    });
  });
}
