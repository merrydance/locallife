package com.merrydance.locallife.merchant.push

import android.content.Context
import android.util.Log
import com.xiaomi.mipush.sdk.MiPushClient
import com.xiaomi.mipush.sdk.MiPushCommandMessage
import com.xiaomi.mipush.sdk.MiPushMessage
import com.xiaomi.mipush.sdk.PushMessageReceiver
import org.json.JSONObject

class XiaomiPushReceiver : PushMessageReceiver() {
    override fun onReceivePassThroughMessage(context: Context, message: MiPushMessage) {
        Log.d("XiaomiPushReceiver", "Received pass-through payload")
        PushManager.onMessageReceived(newPayload(message))
    }

    override fun onNotificationMessageClicked(context: Context, message: MiPushMessage) {
        PushManager.onNotificationOpened(newPayload(message))
    }

    override fun onNotificationMessageArrived(context: Context, message: MiPushMessage) {
        PushManager.onMessageReceived(newPayload(message))
    }

    override fun onCommandResult(context: Context, message: MiPushCommandMessage) {
        val command = message.command
        val arguments = message.commandArguments
        if (MiPushClient.COMMAND_REGISTER == command && arguments != null && arguments.size > 0) {
            val regId = arguments[0]
            PushManager.onTokenRegistered(regId, "xiaomi")
        }
    }

    override fun onReceiveRegisterResult(context: Context, message: MiPushCommandMessage) {
        val command = message.command
        val arguments = message.commandArguments
        if (MiPushClient.COMMAND_REGISTER == command && arguments != null && arguments.isNotEmpty()) {
            PushManager.onTokenRegistered(arguments[0], "xiaomi")
        }
    }

    private fun newPayload(message: MiPushMessage): Map<String, Any> {
        val payload = message.extra.toMutableMap<String, Any>()
        payload.putAll(parseJsonObject(message.content.orEmpty()))
        return payload
    }

    private fun parseJsonObject(raw: String): Map<String, Any> {
        if (raw.isBlank()) return emptyMap()
        return runCatching {
            val json = JSONObject(raw)
            json.keys().asSequence().associateWith { key -> json.get(key) }
        }.getOrDefault(emptyMap())
    }
}
