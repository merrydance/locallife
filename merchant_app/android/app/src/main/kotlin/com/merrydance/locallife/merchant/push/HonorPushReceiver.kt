package com.merrydance.locallife.merchant.push

import android.content.Context
import android.util.Log
import com.hihonor.mcs.push.HonorPushClient
import com.hihonor.mcs.receiver.MessageReceiver

class HonorPushReceiver : MessageReceiver() {
    override fun onToken(context: Context, token: String) {
        PushManager.onTokenRegistered(token, "honor")
    }

    override fun onMessageReceived(context: Context, message: com.hihonor.mcs.model.Message) {
        Log.d("HonorPushReceiver", "Received message: ${message.data}")
        // message.data is the payload
        // Need to convert to Map<String, Any>
        // Depending on SDK version, might need parsing
        val data = mutableMapOf<String, Any>()
        // Assuming data is a simple string for now, but usually it's JSON
        PushManager.onMessageReceived(data)
    }
}
