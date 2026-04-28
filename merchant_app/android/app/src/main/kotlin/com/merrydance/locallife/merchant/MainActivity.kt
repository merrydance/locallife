package com.merrydance.locallife.merchant

import io.flutter.embedding.android.FlutterActivity
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.plugin.common.MethodChannel
import com.merrydance.locallife.merchant.push.PushManager

class MainActivity : FlutterActivity() {
    private val CHANNEL = "com.locallife.merchant/push"

    override fun configureFlutterEngine(flutterEngine: FlutterEngine) {
        super.configureFlutterEngine(flutterEngine)

        val channel = MethodChannel(flutterEngine.dartExecutor.binaryMessenger, CHANNEL)
        PushManager.setChannel(channel)

        channel.setMethodCallHandler { call, result ->
            when (call.method) {
                "initialize" -> {
                    val manufacturer = call.argument<String>("manufacturer") ?: ""
                    PushManager.init(this, manufacturer)
                    result.success(null)
                }
                "getRegistrationId" -> {
                    // Logic to return current cached ID
                    result.success(null)
                }
                else -> {
                    result.notImplemented()
                }
            }
        }
    }
}
