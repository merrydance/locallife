import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter/widgets.dart';
import 'package:merchant_app/core/network/api_client.dart';
import 'package:merchant_app/core/network/api_provider.dart';
import 'package:merchant_app/features/auth/auth_provider.dart';
import 'package:merchant_app/core/utils/error_handler.dart';

class WorkingStatusState {
  static const Object _unset = Object();

  final bool isOnline;
  final bool isLoading;
  final bool isUpdating;
  final bool hasConfirmedState;
  final String? message;
  final String? error;

  const WorkingStatusState({
    this.isOnline = false,
    this.isLoading = false,
    this.isUpdating = false,
    this.hasConfirmedState = false,
    this.message,
    this.error,
  });

  WorkingStatusState copyWith({
    bool? isOnline,
    bool? isLoading,
    bool? isUpdating,
    bool? hasConfirmedState,
    Object? message = _unset,
    Object? error = _unset,
  }) {
    return WorkingStatusState(
      isOnline: isOnline ?? this.isOnline,
      isLoading: isLoading ?? this.isLoading,
      isUpdating: isUpdating ?? this.isUpdating,
      hasConfirmedState: hasConfirmedState ?? this.hasConfirmedState,
      message: identical(message, _unset) ? this.message : message as String?,
      error: identical(error, _unset) ? this.error : error as String?,
    );
  }
}

class WorkingStatusNotifier extends StateNotifier<WorkingStatusState> {
  WorkingStatusNotifier(this._apiClient) : super(const WorkingStatusState());

  final ApiClient _apiClient;
  Future<void>? _syncFuture;
  Future<bool>? _updateFuture;
  int _requestGeneration = 0;

  Future<void> syncFromBackend() {
    final existing = _syncFuture;
    if (existing != null) {
      return existing;
    }

    _syncFuture = _performSyncFromBackend();
    return _syncFuture!.whenComplete(() => _syncFuture = null);
  }

  Future<void> _performSyncFromBackend() async {
    final generation = ++_requestGeneration;
    state = state.copyWith(isLoading: true, error: null);
    try {
      final response = await _apiClient.get('/merchants/me/status');
      if (generation != _requestGeneration) {
        return;
      }
      state = _stateFromResponse(
        response.data,
        fallback: state,
      ).copyWith(isLoading: false, isUpdating: false, hasConfirmedState: true);
    } catch (e) {
      state = state.copyWith(
        isLoading: false,
        error: ErrorHandler.getErrorMessage(e),
      );
    }
  }

  Future<bool> setStatus(bool isOnline) {
    final existing = _updateFuture;
    if (existing != null) {
      return existing;
    }

    _updateFuture = _performSetStatus(isOnline);
    return _updateFuture!.whenComplete(() => _updateFuture = null);
  }

  Future<bool> _performSetStatus(bool isOnline) async {
    final generation = ++_requestGeneration;
    state = state.copyWith(isUpdating: true, error: null);
    try {
      final response = await _apiClient.patch(
        '/merchants/me/status',
        data: <String, dynamic>{'is_open': isOnline},
      );
      if (generation != _requestGeneration) {
        return state.isOnline;
      }
      state = _stateFromResponse(
        response.data,
        fallback: state,
      ).copyWith(isLoading: false, isUpdating: false, hasConfirmedState: true);
      return state.isOnline;
    } catch (e) {
      state = state.copyWith(
        isUpdating: false,
        error: ErrorHandler.getErrorMessage(e),
      );
      return state.isOnline;
    }
  }

  void resetLocal() {
    _requestGeneration += 1;
    state = const WorkingStatusState();
  }

  WorkingStatusState _stateFromResponse(
    dynamic payload, {
    required WorkingStatusState fallback,
  }) {
    final data = extractApiResponseData(payload);
    final isOpen = data['is_open'];
    final message = data['message']?.toString();

    return fallback.copyWith(
      isOnline: isOpen is bool ? isOpen : fallback.isOnline,
      message: message != null && message.isNotEmpty ? message : null,
      error: null,
    );
  }
}

final workingStatusProvider =
    StateNotifierProvider<WorkingStatusNotifier, WorkingStatusState>((ref) {
      final apiClient = ref.watch(apiClientProvider);
      return WorkingStatusNotifier(apiClient);
    });

final workingStatusSyncManagerProvider = Provider<WorkingStatusSyncManager>((
  ref,
) {
  final manager = WorkingStatusSyncManager(ref);
  WidgetsBinding.instance.addObserver(manager);
  ref.listen(authProvider.select((state) => state.isAuthenticated), (
    previous,
    next,
  ) {
    if (next) {
      manager.sync();
    } else {
      ref.read(workingStatusProvider.notifier).resetLocal();
    }
  }, fireImmediately: true);
  ref.onDispose(() => WidgetsBinding.instance.removeObserver(manager));
  return manager;
});

class WorkingStatusSyncManager extends WidgetsBindingObserver {
  WorkingStatusSyncManager(this._ref);

  final Ref _ref;

  void sync() {
    if (_ref.read(authProvider).isAuthenticated) {
      _ref.read(workingStatusProvider.notifier).syncFromBackend();
    }
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.resumed) {
      sync();
    }
  }
}
