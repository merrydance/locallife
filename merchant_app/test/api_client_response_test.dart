import 'package:flutter_test/flutter_test.dart';
import 'package:merchant_app/core/network/api_client.dart';
import 'package:dio/dio.dart';

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

  test('classifies refresh timeout as recoverable', () {
    final result = classifyRefreshFailure(
      DioException(
        requestOptions: RequestOptions(path: '/auth/refresh'),
        type: DioExceptionType.connectionTimeout,
      ),
    );

    expect(result, AuthRefreshFailureKind.recoverable);
  });

  test('classifies refresh 500 as recoverable', () {
    final result = classifyRefreshFailure(
      DioException.badResponse(
        statusCode: 500,
        requestOptions: RequestOptions(path: '/auth/refresh'),
        response: Response<void>(
          requestOptions: RequestOptions(path: '/auth/refresh'),
          statusCode: 500,
        ),
      ),
    );

    expect(result, AuthRefreshFailureKind.recoverable);
  });

  test('classifies refresh 401 as invalid session', () {
    final result = classifyRefreshFailure(
      DioException.badResponse(
        statusCode: 401,
        requestOptions: RequestOptions(path: '/auth/refresh'),
        response: Response<void>(
          requestOptions: RequestOptions(path: '/auth/refresh'),
          statusCode: 401,
        ),
      ),
    );

    expect(result, AuthRefreshFailureKind.invalidSession);
  });
}
