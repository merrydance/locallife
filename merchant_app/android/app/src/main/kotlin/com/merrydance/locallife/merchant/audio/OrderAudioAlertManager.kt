package com.merrydance.locallife.merchant.audio

import android.content.Context
import android.media.AudioAttributes
import android.media.AudioFocusRequest
import android.media.AudioManager
import android.os.Build
import android.os.Bundle
import android.speech.tts.TextToSpeech
import android.speech.tts.UtteranceProgressListener
import android.util.Log
import java.util.Locale
import java.util.UUID
import kotlin.math.ceil
import kotlin.math.max

class OrderAudioAlertManager(context: Context) : TextToSpeech.OnInitListener {
    private val tag = "OrderAudioAlert"
    private val appContext = context.applicationContext
    private val audioManager = context.getSystemService(Context.AUDIO_SERVICE) as AudioManager
    private var sessionDepth = 0
    private var originalAlarmVolume: Int? = null
    private var audioFocusRequest: AudioFocusRequest? = null
    private var tts: TextToSpeech? = TextToSpeech(appContext, this)
    private var ttsReady = false

    override fun onInit(status: Int) {
        ttsReady = status == TextToSpeech.SUCCESS
        if (ttsReady) {
            tts?.language = Locale.SIMPLIFIED_CHINESE
            tts?.setSpeechRate(1.0f)
            tts?.setPitch(1.0f)
            configureTtsAudioAttributes()
        } else {
            Log.w(tag, "TextToSpeech initialization failed with status $status")
        }
    }

    fun beginAlertSession(targetVolume: Double) {
        try {
            if (sessionDepth == 0) {
                originalAlarmVolume = audioManager.getStreamVolume(AudioManager.STREAM_ALARM)
                requestAudioFocus()
                raiseAlarmVolume(targetVolume)
            }
            sessionDepth += 1
        } catch (error: RuntimeException) {
            Log.w(tag, "Failed to begin alert audio session", error)
        }
    }

    fun endAlertSession() {
        try {
            if (sessionDepth <= 0) {
                return
            }

            sessionDepth -= 1
            if (sessionDepth == 0) {
                restoreAlarmVolume()
                abandonAudioFocus()
            }
        } catch (error: RuntimeException) {
            Log.w(tag, "Failed to end alert audio session", error)
            sessionDepth = 0
            originalAlarmVolume = null
        }
    }

    fun speakAlarm(text: String): Boolean {
        if (!ttsReady || tts == null || text.isBlank()) {
            return false
        }

        beginAlertSession(0.8)
        return try {
            configureTtsAudioAttributes()
            val utteranceId = UUID.randomUUID().toString()
            tts?.setOnUtteranceProgressListener(object : UtteranceProgressListener() {
                override fun onStart(utteranceId: String?) = Unit

                override fun onDone(utteranceId: String?) {
                    endAlertSession()
                }

                @Deprecated("Deprecated in Java")
                override fun onError(utteranceId: String?) {
                    endAlertSession()
                }

                override fun onError(utteranceId: String?, errorCode: Int) {
                    endAlertSession()
                }
            })

            val params = Bundle().apply {
                putFloat(TextToSpeech.Engine.KEY_PARAM_VOLUME, 1.0f)
            }
            val result = tts!!.speak(text, TextToSpeech.QUEUE_FLUSH, params, utteranceId)
            if (result == TextToSpeech.SUCCESS) {
                true
            } else {
                endAlertSession()
                false
            }
        } catch (error: RuntimeException) {
            Log.w(tag, "Failed to speak alert with alarm TTS", error)
            endAlertSession()
            false
        }
    }

    private fun raiseAlarmVolume(targetVolume: Double) {
        val maxVolume = audioManager.getStreamMaxVolume(AudioManager.STREAM_ALARM)
        if (maxVolume <= 0) {
            return
        }

        val clampedTarget = targetVolume.coerceIn(0.0, 1.0)
        val targetIndex = ceil(maxVolume * clampedTarget).toInt().coerceIn(1, maxVolume)
        val currentVolume = audioManager.getStreamVolume(AudioManager.STREAM_ALARM)
        val nextVolume = max(currentVolume, targetIndex)
        audioManager.setStreamVolume(AudioManager.STREAM_ALARM, nextVolume, 0)
    }

    private fun restoreAlarmVolume() {
        val previous = originalAlarmVolume ?: return
        val maxVolume = audioManager.getStreamMaxVolume(AudioManager.STREAM_ALARM)
        audioManager.setStreamVolume(
            AudioManager.STREAM_ALARM,
            previous.coerceIn(0, maxVolume),
            0
        )
        originalAlarmVolume = null
    }

    private fun requestAudioFocus() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val attributes = AudioAttributes.Builder()
                .setUsage(AudioAttributes.USAGE_ALARM)
                .setContentType(AudioAttributes.CONTENT_TYPE_SONIFICATION)
                .build()
            val request = AudioFocusRequest.Builder(AudioManager.AUDIOFOCUS_GAIN_TRANSIENT)
                .setAudioAttributes(attributes)
                .setOnAudioFocusChangeListener { }
                .build()
            audioFocusRequest = request
            audioManager.requestAudioFocus(request)
        } else {
            @Suppress("DEPRECATION")
            audioManager.requestAudioFocus(
                null,
                AudioManager.STREAM_ALARM,
                AudioManager.AUDIOFOCUS_GAIN_TRANSIENT
            )
        }
    }

    private fun configureTtsAudioAttributes() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.LOLLIPOP) {
            val attributes = AudioAttributes.Builder()
                .setUsage(AudioAttributes.USAGE_ALARM)
                .setContentType(AudioAttributes.CONTENT_TYPE_SPEECH)
                .build()
            tts?.setAudioAttributes(attributes)
        }
    }

    private fun abandonAudioFocus() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            audioFocusRequest?.let { audioManager.abandonAudioFocusRequest(it) }
            audioFocusRequest = null
        } else {
            @Suppress("DEPRECATION")
            audioManager.abandonAudioFocus(null)
        }
    }

    fun shutdown() {
        try {
            tts?.stop()
            tts?.shutdown()
        } catch (error: RuntimeException) {
            Log.w(tag, "Failed to shutdown alert TTS", error)
        } finally {
            tts = null
            ttsReady = false
            sessionDepth = 0
            originalAlarmVolume = null
            abandonAudioFocus()
        }
    }
}
