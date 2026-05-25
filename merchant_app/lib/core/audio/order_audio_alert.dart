import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';

class OrderAudioChannels {
  static const MethodChannel audioAlert = MethodChannel(
    'com.locallife.merchant/audio_alert',
  );
}

abstract class OrderAudioControl {
  Future<void> beginAlertSession({double targetVolume = 0.8});

  Future<void> endAlertSession();
}

class NativeOrderAudioControl implements OrderAudioControl {
  @override
  Future<void> beginAlertSession({double targetVolume = 0.8}) async {
    try {
      await OrderAudioChannels.audioAlert.invokeMethod<void>(
        'beginAlertSession',
        {'targetVolume': targetVolume},
      );
    } on PlatformException catch (error) {
      debugPrint('Failed to begin order alert audio session: ${error.message}');
    } on MissingPluginException {
      debugPrint('Order alert audio session plugin is unavailable.');
    }
  }

  @override
  Future<void> endAlertSession() async {
    try {
      await OrderAudioChannels.audioAlert.invokeMethod<void>('endAlertSession');
    } on PlatformException catch (error) {
      debugPrint('Failed to end order alert audio session: ${error.message}');
    } on MissingPluginException {
      debugPrint('Order alert audio session plugin is unavailable.');
    }
  }
}

abstract class OrderAudioSpeech {
  Future<bool> speak(String text);
}

class NativeOrderAudioSpeech implements OrderAudioSpeech {
  @override
  Future<bool> speak(String text) async {
    try {
      final spoken = await OrderAudioChannels.audioAlert.invokeMethod<bool>(
        'speakAlarm',
        {'text': text},
      );
      return spoken == true;
    } on PlatformException catch (error) {
      debugPrint(
        'Failed to speak order alert with native TTS: ${error.message}',
      );
      return false;
    } on MissingPluginException {
      debugPrint('Order alert native TTS plugin is unavailable.');
      return false;
    }
  }
}

class OrderAudioAlert {
  OrderAudioAlert({OrderAudioControl? audioControl})
    : _audioControl = audioControl ?? NativeOrderAudioControl();

  final OrderAudioControl _audioControl;

  Future<T> runWithAlertVolume<T>(Future<T> Function() action) async {
    await _audioControl.beginAlertSession();
    try {
      return await action();
    } finally {
      await _audioControl.endAlertSession();
    }
  }
}
