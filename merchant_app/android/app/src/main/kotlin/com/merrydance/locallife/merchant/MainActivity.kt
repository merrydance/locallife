package com.merrydance.locallife.merchant

import io.flutter.embedding.android.FlutterActivity
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.plugin.common.MethodChannel
import com.merrydance.locallife.merchant.audio.OrderAudioAlertManager
import com.merrydance.locallife.merchant.push.PushManager

class MainActivity : FlutterActivity() {
    private val PUSH_CHANNEL = "com.locallife.merchant/push"
    private val AUDIO_ALERT_CHANNEL = "com.locallife.merchant/audio_alert"
    private var audioAlertManager: OrderAudioAlertManager? = null

    override fun configureFlutterEngine(flutterEngine: FlutterEngine) {
        super.configureFlutterEngine(flutterEngine)

        val pushChannel = MethodChannel(flutterEngine.dartExecutor.binaryMessenger, PUSH_CHANNEL)
        PushManager.setChannel(pushChannel)

        pushChannel.setMethodCallHandler { call, result ->
            when (call.method) {
                "initialize" -> {
                    val manufacturer = call.argument<String>("manufacturer") ?: ""
                    PushManager.init(this, manufacturer)
                    result.success(null)
                }
                "getRegistrationId" -> {
                    result.success(PushManager.getRegistrationId())
                }
                else -> {
                    result.notImplemented()
                }
            }
        }

        audioAlertManager = OrderAudioAlertManager(applicationContext)
        val audioAlertChannel = MethodChannel(
            flutterEngine.dartExecutor.binaryMessenger,
            AUDIO_ALERT_CHANNEL
        )

        audioAlertChannel.setMethodCallHandler { call, result ->
            when (call.method) {
                "beginAlertSession" -> {
                    val targetVolume = call.argument<Double>("targetVolume") ?: 0.8
                    audioAlertManager?.beginAlertSession(targetVolume)
                    result.success(null)
                }
                "endAlertSession" -> {
                    audioAlertManager?.endAlertSession()
                    result.success(null)
                }
                "speakAlarm" -> {
                    val text = call.argument<String>("text") ?: ""
                    result.success(audioAlertManager?.speakAlarm(text) == true)
                }
                else -> {
                    result.notImplemented()
                }
            }
        }
    }

    override fun cleanUpFlutterEngine(flutterEngine: FlutterEngine) {
        audioAlertManager?.shutdown()
        audioAlertManager = null
        super.cleanUpFlutterEngine(flutterEngine)
    }
}
