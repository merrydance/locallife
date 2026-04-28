import 'package:dio/dio.dart';

class ErrorHandler {
  static String getErrorMessage(dynamic error) {
    if (error is DioException) {
      switch (error.type) {
        case DioExceptionType.connectionTimeout:
        case DioExceptionType.sendTimeout:
        case DioExceptionType.receiveTimeout:
          return '网络连接超时，请检查网络后再试';
        case DioExceptionType.badResponse:
          final statusCode = error.response?.statusCode;
          final data = error.response?.data;

          if (data is Map && data.containsKey('message')) {
            final message = data['message'];
            if (message == '绑定码无效或已过期') return message;
            if (message == 'access token is not provided') return '未提供登录凭证';
          }

          switch (statusCode) {
            case 400:
              return '请求参数错误';
            case 401:
              return '登录权限已失效，请重新登录';
            case 403:
              return '权限不足，无法访问';
            case 404:
              return '请求的资源不存在';
            case 500:
              return '服务器内部错误';
            default:
              return '服务器响应异常 ($statusCode)';
          }
        case DioExceptionType.cancel:
          return '请求已取消';
        case DioExceptionType.connectionError:
          return '无法连接到服务器，请检查网络';
        default:
          return '网络请求失败，请稍后重试';
      }
    }
    return error.toString();
  }
}
