import 'package:flutter_test/flutter_test.dart';
import 'package:flutter/services.dart';
import 'package:merchant_app/core/audio/order_audio_alert.dart';
import 'package:merchant_app/core/audio/tts_service.dart';

void main() {
  test('wraps alert playback in an elevated alarm volume session', () async {
    final audioControl = _FakeOrderAudioControl();
    final events = <String>[];

    await OrderAudioAlert(audioControl: audioControl).runWithAlertVolume(
      () async {
        events.add('play');
      },
    );

    expect(events, ['play']);
    expect(audioControl.calls, ['begin', 'end']);
  });

  test('restores alarm volume when playback throws', () async {
    final audioControl = _FakeOrderAudioControl();

    await expectLater(
      OrderAudioAlert(
        audioControl: audioControl,
      ).runWithAlertVolume(() async => throw StateError('playback failed')),
      throwsStateError,
    );

    expect(audioControl.calls, ['begin', 'end']);
  });

  test(
    'sends native alarm TTS requests over the audio alert channel',
    () async {
      TestWidgetsFlutterBinding.ensureInitialized();
      final calls = <MethodCall>[];
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(OrderAudioChannels.audioAlert, (
            call,
          ) async {
            calls.add(call);
            return true;
          });
      addTearDown(() {
        TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
            .setMockMethodCallHandler(OrderAudioChannels.audioAlert, null);
      });

      final spoken = await NativeOrderAudioSpeech().speak(
        '订单 A001 号，金额 18.50 元',
      );

      expect(spoken, isTrue);
      expect(calls, hasLength(1));
      expect(calls.single.method, 'speakAlarm');
      expect(calls.single.arguments, <String, dynamic>{
        'text': '订单 A001 号，金额 18.50 元',
      });
    },
  );

  test('uses fixed merchant new order alert copy', () {
    expect(TtsService.newOrderAlertText, '您有乐客来福新外卖单了');
  });
}

class _FakeOrderAudioControl implements OrderAudioControl {
  final List<String> calls = [];

  @override
  Future<void> beginAlertSession({double targetVolume = 0.8}) async {
    calls.add('begin');
  }

  @override
  Future<void> endAlertSession() async {
    calls.add('end');
  }
}
