import 'package:flutter/foundation.dart';
import 'package:flutter_tts/flutter_tts.dart';
import 'package:merchant_app/core/audio/order_audio_alert.dart';

class TtsService {
  static const String newOrderAlertText = '您有乐客来福新外卖单了';

  static final FlutterTts _flutterTts = FlutterTts();
  static final OrderAudioSpeech _nativeSpeech = NativeOrderAudioSpeech();

  static Future<void> init() async {
    await _flutterTts.setLanguage("zh-CN");
    await _flutterTts.setSpeechRate(0.5);
    await _flutterTts.setVolume(1.0);
    await _flutterTts.setPitch(1.0);
    await _flutterTts.awaitSpeakCompletion(true);
    await _flutterTts.setAudioAttributesForNavigation();
  }

  static Future<void> speak(String text) async {
    try {
      if (await _nativeSpeech.speak(text)) {
        return;
      }
      await _flutterTts.setVolume(1.0);
      await _flutterTts.speak(text, focus: true);
    } catch (e) {
      debugPrint('TTS error: $e');
    }
  }

  // Kept for API compatibility with callers that still pass order details.
  // ignore: avoid-unused-parameters
  static Future<void> speakOrderAlert(String orderNum, double amount) async {
    await speak(newOrderAlertText);
  }
}
