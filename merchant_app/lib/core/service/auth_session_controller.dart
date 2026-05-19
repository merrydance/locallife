import 'package:flutter/foundation.dart';

class AuthSessionController extends ChangeNotifier {
  String? _lastInvalidationReason;
  String? _latestAccessToken;
  String? _latestRefreshToken;

  String? get lastInvalidationReason => _lastInvalidationReason;
  String? get latestAccessToken => _latestAccessToken;
  String? get latestRefreshToken => _latestRefreshToken;

  void invalidate([String? reason]) {
    _lastInvalidationReason = reason;
    notifyListeners();
  }

  void updateTokens({
    required String accessToken,
    required String refreshToken,
  }) {
    _latestAccessToken = accessToken;
    _latestRefreshToken = refreshToken;
    notifyListeners();
  }

  void clearInvalidation() {
    _lastInvalidationReason = null;
  }

  void clearTokenUpdate() {
    _latestAccessToken = null;
    _latestRefreshToken = null;
  }
}
