package com.merrydance.locallife.merchant.push

import android.util.Log
import com.hihonor.push.sdk.HonorMessageService
import com.hihonor.push.sdk.HonorPushDataMsg
import org.json.JSONObject

class HonorPushReceiver : HonorMessageService() {
    override fun onNewToken(token: String) {
        PushManager.onTokenRegistered(token, "honor")
    }

    override fun onMessageReceived(message: HonorPushDataMsg) {
        val data = parseJsonObject(message.data.orEmpty())
        Log.d("HonorPushReceiver", "Received payload keys: ${data.keys}")
        if (data.isNotEmpty()) {
            PushManager.onMessageReceived(data)
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
