package com.merrydance.locallife.merchant.push

import android.content.Context
import android.util.Log
import com.vivo.push.model.UPSNotificationMessage
import com.vivo.push.sdk.PushMessageReceiver

class VivoPushReceiver : PushMessageReceiver() {
    override fun onTransmissionMessage(context: Context, message: UPSNotificationMessage) {
        // Pass-through message
        Log.d("VivoPushReceiver", "Received: ${message.content}")
        // Parse message.params or message.content
        PushManager.onMessageReceived(message.params)
    }

    override fun onNotificationMessageClicked(context: Context, message: UPSNotificationMessage) {
        // Handle click
    }

    override fun onReceiveRegId(context: Context, regId: String) {
        PushManager.onTokenRegistered(regId, "vivo")
    }
}
