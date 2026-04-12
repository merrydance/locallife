import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

class NotificationSettingsState {
  const NotificationSettingsState({
    this.soundEnabled = true,
    this.voiceEnabled = true,
    this.autoAcceptEnabled = false,
    this.autoPrintAfterAcceptEnabled = true,
  });

  final bool soundEnabled;
  final bool voiceEnabled;
  final bool autoAcceptEnabled;
  final bool autoPrintAfterAcceptEnabled;

  NotificationSettingsState copyWith({
    bool? soundEnabled,
    bool? voiceEnabled,
    bool? autoAcceptEnabled,
    bool? autoPrintAfterAcceptEnabled,
  }) {
    return NotificationSettingsState(
      soundEnabled: soundEnabled ?? this.soundEnabled,
      voiceEnabled: voiceEnabled ?? this.voiceEnabled,
      autoAcceptEnabled: autoAcceptEnabled ?? this.autoAcceptEnabled,
      autoPrintAfterAcceptEnabled:
          autoPrintAfterAcceptEnabled ?? this.autoPrintAfterAcceptEnabled,
    );
  }
}

class NotificationSettingsNotifier
    extends StateNotifier<NotificationSettingsState> {
  NotificationSettingsNotifier() : super(const NotificationSettingsState()) {
    _load();
  }

  static const _soundKey = 'notification_sound_enabled';
  static const _voiceKey = 'notification_voice_enabled';
  static const _autoAcceptKey = 'order_auto_accept_enabled';
  static const _autoPrintAfterAcceptKey = 'order_auto_print_after_accept_enabled';

  Future<void> _load() async {
    final prefs = await SharedPreferences.getInstance();
    state = state.copyWith(
      soundEnabled: prefs.getBool(_soundKey) ?? true,
      voiceEnabled: prefs.getBool(_voiceKey) ?? true,
      autoAcceptEnabled: prefs.getBool(_autoAcceptKey) ?? false,
      autoPrintAfterAcceptEnabled:
          prefs.getBool(_autoPrintAfterAcceptKey) ?? true,
    );
  }

  Future<void> setSoundEnabled(bool enabled) async {
    state = state.copyWith(soundEnabled: enabled);
    final prefs = await SharedPreferences.getInstance();
    await prefs.setBool(_soundKey, enabled);
  }

  Future<void> setVoiceEnabled(bool enabled) async {
    state = state.copyWith(voiceEnabled: enabled);
    final prefs = await SharedPreferences.getInstance();
    await prefs.setBool(_voiceKey, enabled);
  }

  Future<void> setAutoAcceptEnabled(bool enabled) async {
    state = state.copyWith(autoAcceptEnabled: enabled);
    final prefs = await SharedPreferences.getInstance();
    await prefs.setBool(_autoAcceptKey, enabled);
  }

  Future<void> setAutoPrintAfterAcceptEnabled(bool enabled) async {
    state = state.copyWith(autoPrintAfterAcceptEnabled: enabled);
    final prefs = await SharedPreferences.getInstance();
    await prefs.setBool(_autoPrintAfterAcceptKey, enabled);
  }
}

final notificationSettingsProvider = StateNotifierProvider<
    NotificationSettingsNotifier, NotificationSettingsState>((ref) {
  return NotificationSettingsNotifier();
});