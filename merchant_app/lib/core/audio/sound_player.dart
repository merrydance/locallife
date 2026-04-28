import 'package:flutter/foundation.dart';
import 'package:audioplayers/audioplayers.dart';

class SoundPlayer {
  static final AudioPlayer _player = AudioPlayer();

  static Future<void> playAsset(String path) async {
    try {
      await _player.stop();
      await _player.play(AssetSource(path));
    } catch (e) {
      debugPrint('Error playing sound: $e');
    }
  }

  static Future<void> playNewOrderAlert() async {
    // This will play assets/audio/new_order.mp3
    await playAsset('audio/new_order.mp3');
  }

  static void dispose() {
    _player.dispose();
  }
}
