package com.merrydance.locallife.merchant.push

import android.content.Context
import android.content.pm.PackageManager
import android.util.Log
import com.heytap.msp.push.HeytapPushManager
import com.hihonor.push.sdk.HonorPushCallback
import com.hihonor.push.sdk.HonorPushClient
import com.vivo.push.IPushActionListener
import com.vivo.push.PushClient
import com.vivo.push.PushConfig
import com.vivo.push.listener.IPushQueryActionListener
import com.vivo.push.util.VivoPushException
import com.xiaomi.mipush.sdk.MiPushClient
import io.flutter.plugin.common.MethodChannel

object PushManager {
    private const val TAG = "PushManager"
    private var channel: MethodChannel? = null
    private var context: Context? = null
    private var registrationId: String? = null
    private var registrationProvider: String? = null

    fun setChannel(channel: MethodChannel) {
        this.channel = channel
    }

    fun init(context: Context, manufacturer: String) {
        this.context = context.applicationContext
        val normalizedManufacturer = manufacturer.lowercase()

        Log.d(TAG, "Initializing push for manufacturer: $normalizedManufacturer")

        when {
            normalizedManufacturer.contains("xiaomi") -> initXiaomi(context)
            normalizedManufacturer.contains("vivo") -> initVivo(context)
            normalizedManufacturer.contains("oppo") || normalizedManufacturer.contains("realme") -> initOppo(context)
            normalizedManufacturer.contains("honor") -> initHonor(context)
            else -> Log.i(TAG, "No native push provider configured for manufacturer: $normalizedManufacturer")
        }
    }

    fun onTokenRegistered(token: String, provider: String) {
        registrationId = token
        registrationProvider = provider
        Log.d(TAG, "Push token registered for provider: $provider")
        channel?.invokeMethod(
            "onTokenRegistered",
            mapOf(
                "token" to token,
                "provider" to provider,
            ),
        )
    }

    fun onMessageReceived(message: Map<String, Any>) {
        Log.d(TAG, "Push message received")
        channel?.invokeMethod("onReceiveMessage", message)
    }

    fun getRegistrationId(): String? = registrationId

    fun getRegistrationProvider(): String? = registrationProvider

    private fun initXiaomi(context: Context) {
        val appId = getMetadata(context, "XIAOMI_APP_ID")
        val appKey = getMetadata(context, "XIAOMI_APP_KEY")
        if (appId != null && appKey != null) {
            MiPushClient.registerPush(context, appId, appKey)
            MiPushClient.getRegId(context).takeIf { it.isNotBlank() }?.let {
                onTokenRegistered(it, "xiaomi")
            }
        }
    }

    private fun initVivo(context: Context) {
        val client = PushClient.getInstance(context)
        try {
            client.initialize(PushConfig.Builder().agreePrivacyStatement(true).build())
        } catch (e: VivoPushException) {
            Log.w(TAG, "vivo push initialization failed", e)
        }
        client.turnOnPush(object : IPushActionListener {
            override fun onStateChanged(state: Int) {
                if (state == 0) {
                    client.getRegId(object : IPushQueryActionListener {
                        override fun onSuccess(regId: String) {
                            onTokenRegistered(regId, "vivo")
                        }

                        override fun onFail(errorCode: Int?) {
                            Log.w(TAG, "vivo push regId query failed: $errorCode")
                        }
                    })
                } else {
                    Log.w(TAG, "vivo push turnOnPush failed: $state")
                }
            }
        })
    }

    private fun initOppo(context: Context) {
        HeytapPushManager.init(context, true)
        val appKey = getMetadata(context, "OPPO_APP_KEY")
        val appSecret = getMetadata(context, "OPPO_APP_SECRET")
        if (appKey != null && appSecret != null) {
            HeytapPushManager.register(
                context,
                appKey,
                appSecret,
                object : com.heytap.msp.push.callback.ICallBackResultService {
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
                },
            )
        }
    }

    private fun initHonor(context: Context) {
        HonorPushClient.getInstance().init(context, true)
        HonorPushClient.getInstance().getPushToken(object : HonorPushCallback<String> {
            override fun onSuccess(token: String) {
                onTokenRegistered(token, "honor")
            }

            override fun onFailure(errorCode: Int, errorMessage: String) {
                Log.w(TAG, "Honor push token registration failed: $errorCode $errorMessage")
            }
        })
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
