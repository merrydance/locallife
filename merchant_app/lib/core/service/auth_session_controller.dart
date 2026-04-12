import 'package:flutter/foundation.dart';

class AuthSessionController extends ChangeNotifier {
  String? _lastInvalidationReason;

  String? get lastInvalidationReason => _lastInvalidationReason;

  void invalidate([String? reason]) {
    _lastInvalidationReason = reason;
    notifyListeners();
  }

  void clearInvalidation() {
    _lastInvalidationReason = null;
  }
}