class AuthState {
  static const Object _unset = Object();

  final String? accessToken;
  final String? refreshToken;
  final String? merchantName;
  final bool isAuthenticated;
  final bool isLoading;
  final bool isSessionDegraded;
  final String? error;

  AuthState({
    this.accessToken,
    this.refreshToken,
    this.merchantName,
    this.isAuthenticated = false,
    this.isLoading = false,
    this.isSessionDegraded = false,
    this.error,
  });

  AuthState copyWith({
    Object? accessToken = _unset,
    Object? refreshToken = _unset,
    Object? merchantName = _unset,
    bool? isAuthenticated,
    bool? isLoading,
    bool? isSessionDegraded,
    Object? error = _unset,
  }) {
    return AuthState(
      accessToken: identical(accessToken, _unset)
          ? this.accessToken
          : accessToken as String?,
      refreshToken: identical(refreshToken, _unset)
          ? this.refreshToken
          : refreshToken as String?,
      merchantName: identical(merchantName, _unset)
          ? this.merchantName
          : merchantName as String?,
      isAuthenticated: isAuthenticated ?? this.isAuthenticated,
      isLoading: isLoading ?? this.isLoading,
      isSessionDegraded: isSessionDegraded ?? this.isSessionDegraded,
      error: identical(error, _unset) ? this.error : error as String?,
    );
  }
}
