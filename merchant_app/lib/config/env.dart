class Env {
  static const String apiBaseUrl = String.fromEnvironment(
    'API_BASE_URL',
    defaultValue: 'https://llapi.merrydance.cn/v1',
  );

  static const String wsUrl = String.fromEnvironment(
    'WS_URL',
    defaultValue: 'wss://llapi.merrydance.cn/v1/ws',
  );

  static const bool isDebug = !bool.fromEnvironment('dart.vm.product');
}
