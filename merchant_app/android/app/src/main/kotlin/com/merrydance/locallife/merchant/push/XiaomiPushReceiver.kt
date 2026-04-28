package com.merrydance.locallife.merchant.push

import android.content.Context
import android.util.Log
import com.xiaomi.mipush.sdk.MiPushCommandMessage
import com.xiaomi.mipush.sdk.MiPushMessage
import com.xiaomi.mipush.sdk.PushMessageReceiver

class XiaomiPushReceiver : PushMessageReceiver() {
    override fun onReceivePassThroughMessage(context: Context, message: MiPushMessage) {
        val payload = message.content
        Log.d("XiaomiPushReceiver", "Received payload: $payload")
        // Typically messages are JSON string in 'content'
        // For simplicity, we pass extras as the map
        PushManager.onMessageReceived(message.extra.mapValues { it.value })
    }

    override fun onNotificationMessageClicked(context: Context, message: MiPushMessage) {
        // Handle click if needed
    }

    override fun onCommandResult(context: Context, message: MiPushCommandMessage) {
        val command = message.command
        val arguments = message.commandArguments
        if (MiPushClient.COMMAND_REGISTER == command && arguments != null && arguments.size > 0) {
            val regId = arguments[0]
            PushManager.onTokenRegistered(regId, "xiaomi")
        }
    }
}
