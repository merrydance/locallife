package com.merrydance.locallife.merchant.push

import android.content.Context
import android.util.Log
import com.vivo.push.model.UPSNotificationMessage
import com.vivo.push.model.UnvarnishedMessage
import com.vivo.push.sdk.OpenClientPushMessageReceiver
import org.json.JSONObject

class VivoPushReceiver : OpenClientPushMessageReceiver() {
    override fun onReceiveRegId(context: Context, regId: String) {
        PushManager.onTokenRegistered(regId, "vivo")
    }

    override fun onTransmissionMessage(context: Context, message: UnvarnishedMessage) {
        val payload = message.params.toMutableMap<String, Any>()
        payload.putAll(parseJsonObject(message.message.orEmpty()))
        if (payload.isNotEmpty()) {
            Log.d("VivoPushReceiver", "Received transmission payload keys: ${payload.keys}")
            PushManager.onMessageReceived(payload)
        }
    }

    override fun onNotificationMessageClicked(context: Context, message: UPSNotificationMessage) {
        val payload = message.params.toMutableMap<String, Any>()
        payload.putAll(parseJsonObject(message.content.orEmpty()))
        if (payload.isNotEmpty()) {
            PushManager.onNotificationOpened(payload)
        }
    }

    override fun onForegroundMessageArrived(message: UPSNotificationMessage) {
        val payload = message.params.toMutableMap<String, Any>()
        payload.putAll(parseJsonObject(message.content.orEmpty()))
        if (payload.isNotEmpty()) {
            PushManager.onMessageReceived(payload)
        }
    }

    private fun parseJsonObject(raw: String): Map<String, Any> {
        if (raw.isBlank()) return emptyMap()
        return runCatching {
            val json = JSONObject(raw)
            json.keys().asSequence().associateWith { key -> json.get(key) }
        }.getOrDefault(emptyMap())
    }
}
