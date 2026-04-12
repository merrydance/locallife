import 'package:flutter_riverpod/flutter_riverpod.dart';

class WorkingStatusNotifier extends StateNotifier<bool> {
  WorkingStatusNotifier() : super(false); // Default offline

  void setStatus(bool isOnline) {
    state = isOnline;
  }

  void toggle() {
    state = !state;
  }
}

final workingStatusProvider = StateNotifierProvider<WorkingStatusNotifier, bool>((ref) {
  return WorkingStatusNotifier();
});
