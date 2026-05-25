import 'package:flutter/foundation.dart';
import 'package:flutter_tts/flutter_tts.dart';
import 'package:merchant_app/core/audio/order_audio_alert.dart';

class TtsService {
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

  static Future<void> speakOrderAlert(String orderNum, double amount) async {
    final text = "订单 $orderNum 号，金额 ${amount.toStringAsFixed(2)} 元";
    await speak(text);
  }
}
