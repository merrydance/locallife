package com.merrydance.locallife.merchant.push

import android.content.Context
import android.content.pm.PackageManager
import android.os.Build
import android.util.Log
import io.flutter.plugin.common.MethodChannel
import com.xiaomi.mipush.sdk.MiPushClient
import com.vivo.push.PushClient
import com.heytap.msp.push.HeytapPushManager
import com.hihonor.mcs.push.HonorPushClient

object PushManager {
    private const val TAG = "PushManager"
    private var channel: MethodChannel? = null
    private var context: Context? = null

    fun setChannel(channel: MethodChannel) {
        this.channel = channel
    }

    fun init(context: Context, manufacturer: String) {
        this.context = context.applicationContext

        Log.d(TAG, "Initializing push for manufacturer: $manufacturer")

        when {
            manufacturer.contains("xiaomi") -> {
                val appId = getMetadata(context, "XIAOMI_APP_ID")
                val appKey = getMetadata(context, "XIAOMI_APP_KEY")
                if (appId != null && appKey != null) {
                    MiPushClient.registerPush(context, appId, appKey)
                }
            }
            manufacturer.contains("vivo") -> {
                PushClient.getInstance(context).initialize()
                PushClient.getInstance(context).turnOnPush { state ->
                    if (state == 0) {
                        val regId = PushClient.getInstance(context).regId
                        onTokenRegistered(regId, "vivo")
                    }
                }
            }
            manufacturer.contains("oppo") || manufacturer.contains("realme") -> {
                HeytapPushManager.init(context, true)
                val appKey = getMetadata(context, "OPPO_APP_KEY")
                val appSecret = getMetadata(context, "OPPO_APP_SECRET")
                if (appKey != null && appSecret != null) {
                    HeytapPushManager.register(context, appKey, appSecret, object : com.heytap.msp.push.callback.ICallBackResultService {
                        override fun onRegister(code: Int, regId: String?) {
                            if (code == 0 && regId != null) {
                                onTokenRegistered(regId, "oppo")
                            }
                        }
                        override fun onUnRegister(code: Int) {}
                        override fun onSetPushTime(code: Int, s: String?) {}
                        override fun onGetPushStatus(code: Int, status: Int) {}
                        override fun onGetNotificationStatus(code: Int, status: Int) {}
                        override fun onError(code: Int, s: String?) {}
                    })
                }
            }
            manufacturer.contains("honor") -> {
                HonorPushClient.getInstance().init(context, true)
            }
        }
    }

    fun onTokenRegistered(token: String, provider: String) {
        Log.d(TAG, "Push token registered for provider: $provider")
        channel?.invokeMethod("onTokenRegistered", mapOf(
            "token" to token,
            "provider" to provider
        ))
    }

    fun onMessageReceived(message: Map<String, Any>) {
        Log.d(TAG, "Push message received")
        channel?.invokeMethod("onReceiveMessage", message)
    }

    private fun getMetadata(context: Context, key: String): String? {
        return try {
            val appInfo = context.packageManager.getApplicationInfo(context.packageName, PackageManager.GET_META_DATA)
            appInfo.metaData.get(key)?.toString()
        } catch (e: Exception) {
            null
        }
    }
}
