package dev.openhongguo.app

import android.content.Context
import android.graphics.Color
import android.graphics.SurfaceTexture
import android.media.AudioAttributes
import android.media.AudioFocusRequest
import android.media.AudioManager
import android.os.Handler
import android.os.Looper
import android.os.SystemClock
import android.util.Log
import android.view.Surface
import android.view.TextureView
import android.view.View
import android.widget.FrameLayout
import com.longbridge.nsh.NshComponent
import com.longbridge.nsh.NshComponentContext
import dev.jdtech.mpv.MPVLib
import dev.jdtech.mpv.MPVLib.MpvEvent
import dev.jdtech.mpv.MPVLib.MpvFormat
import dev.jdtech.mpv.MPVLib.MpvLogLevel

/**
 * `ohg.player`: libmpv with the desktop's options (desktop/src/player/engine.rs) drawing into a TextureView.
 * Protected streams are CENC MP4 that libavformat decrypts with `decryption_key`, and the audio is a
 * separate file added once the video has loaded.
 *
 * Props: `source` {url, audioUrl, key, start} or null, `paused`, `rate`.
 * Command: `seek` {position}.
 * Events: `load` {duration}, `progress` {position, duration, buffered}, `pause` {paused},
 * `buffering` {buffering}, `seeked` {position}, `ended`, `error` {message}.
 */
class PlayerComponent(
    private val ctx: NshComponentContext,
    private val players: MutableSet<PlayerComponent>,
) : NshComponent, TextureView.SurfaceTextureListener, MPVLib.EventObserver, MPVLib.LogObserver {
    private val main = Handler(Looper.getMainLooper())
    private val context: Context = ctx.context
    private val texture = TextureView(context)
    private val root = FrameLayout(context)
    private val mpv = checkNotNull(MPVLib.create(context)) { "libmpv did not start" }
    private val audio = context.getSystemService(AudioManager::class.java)
    private val focus = AudioFocusRequest.Builder(AudioManager.AUDIOFOCUS_GAIN)
        .setAudioAttributes(
            AudioAttributes.Builder()
                .setUsage(AudioAttributes.USAGE_MEDIA)
                .setContentType(AudioAttributes.CONTENT_TYPE_MOVIE)
                .build(),
        )
        .setOnAudioFocusChangeListener { change -> if (change < 0) mpv.setPropertyBoolean("pause", true) }
        .build()

    private var surface: Surface? = null
    private var destroyed = false

    // Touched on the main thread only.
    private var pending: Map<*, *>? = null
    private var load = Load.Idle
    private var audioUrl = ""
    private var seeking = false
    private var lastProgress = 0L
    private var paused = false

    // Written on mpv's event thread, read on the main thread.
    @Volatile private var position = 0.0
    @Volatile private var duration = 0.0
    @Volatile private var buffered = 0.0
    @Volatile private var lastError = ""

    private enum class Load { Idle, Requested, Started, Playing }

    init {
        root.setBackgroundColor(Color.BLACK)
        root.addView(texture, FrameLayout.LayoutParams(FrameLayout.LayoutParams.MATCH_PARENT, FrameLayout.LayoutParams.MATCH_PARENT))
        texture.surfaceTextureListener = this
        options()
        mpv.init()
        // The window comes with the surface.
        mpv.setOptionString("force-window", "no")
        for ((name, format) in OBSERVED) mpv.observeProperty(name, format)
        mpv.addObserver(this)
        mpv.addLogObserver(this)
        players += this
    }

    private fun options() {
        mpv.setOptionString("vo", "gpu")
        mpv.setOptionString("gpu-context", "android")
        mpv.setOptionString("opengl-es", "yes")
        mpv.setOptionString("hwdec", "mediacodec-copy")
        mpv.setOptionString("ao", "audiotrack,opensles")
        mpv.setOptionString("keep-open", "yes")
        mpv.setOptionString("idle", "yes")
        mpv.setOptionString("input-default-bindings", "no")
        mpv.setOptionString("ytdl", "no")
        mpv.setOptionString("cache", "yes")
        mpv.setOptionString("demuxer-max-bytes", "150MiB")
        mpv.setOptionString("demuxer-readahead-secs", "30")
        mpv.setOptionString("save-position-on-quit", "no")
    }

    override fun view(): View = root

    override fun setProp(name: String, value: Any?) {
        when (name) {
            "source" -> {
                pending = value as Map<*, *>?
                if (pending == null) stop() else if (surface != null) startPending()
            }
            "paused" -> {
                paused = value as Boolean? ?: false
                mpv.setPropertyBoolean("pause", paused)
                if (!paused) audio.requestAudioFocus(focus)
                root.keepScreenOn = !paused
            }
            "rate" -> mpv.setPropertyDouble("speed", (value as Number?)?.toDouble() ?: 1.0)
        }
    }

    override fun command(name: String, args: Any?) {
        when (name) {
            "seek" -> {
                val at = ((args as Map<*, *>)["position"] as Number).toDouble()
                seeking = true
                mpv.command(arrayOf("seek", "%.3f".format(java.util.Locale.ROOT, at), "absolute"))
            }
            else -> throw IllegalArgumentException("ohg.player has no command $name")
        }
    }

    private fun startPending() {
        val src = pending ?: return
        pending = null
        val key = src["key"] as String? ?: ""
        val start = (src["start"] as Number?)?.toDouble() ?: 0.0
        mpv.setPropertyString("demuxer-lavf-o", if (key.isEmpty()) "" else "decryption_key=$key")
        mpv.setPropertyString("start", if (start > 0) "%.3f".format(java.util.Locale.ROOT, start) else "none")
        audioUrl = src["audioUrl"] as String? ?: ""
        position = start
        duration = 0.0
        buffered = 0.0
        lastError = ""
        load = Load.Requested
        if (!paused) audio.requestAudioFocus(focus)
        mpv.command(arrayOf("loadfile", src["url"] as String, "replace"))
    }

    private fun stop() {
        load = Load.Idle
        mpv.command(arrayOf("stop"))
        audio.abandonAudioFocusRequest(focus)
        root.keepScreenOn = false
    }

    fun pauseForBackground() {
        mpv.setPropertyBoolean("pause", true)
    }

    override fun destroy() {
        destroyed = true
        players -= this
        main.removeCallbacksAndMessages(null)
        audio.abandonAudioFocusRequest(focus)
        mpv.removeObserver(this)
        mpv.removeLogObserver(this)
        detach()
        mpv.destroy()
    }

    override fun onSurfaceTextureAvailable(st: SurfaceTexture, width: Int, height: Int) {
        val s = Surface(st)
        surface = s
        mpv.attachSurface(s)
        mpv.setOptionString("force-window", "yes")
        mpv.setPropertyString("android-surface-size", "${width}x$height")
        mpv.setPropertyString("vo", "gpu")
        startPending()
    }

    override fun onSurfaceTextureSizeChanged(st: SurfaceTexture, width: Int, height: Int) {
        mpv.setPropertyString("android-surface-size", "${width}x$height")
    }

    override fun onSurfaceTextureDestroyed(st: SurfaceTexture): Boolean {
        if (!destroyed) detach()
        return true
    }

    override fun onSurfaceTextureUpdated(st: SurfaceTexture) {}

    private fun detach() {
        val s = surface ?: return
        surface = null
        mpv.setPropertyString("vo", "null")
        mpv.setOptionString("force-window", "no")
        mpv.detachSurface()
        s.release()
    }

    // mpv's event thread from here on: everything that reaches nsh is posted to the main thread.

    override fun event(eventId: Int) {
        when (eventId) {
            MpvEvent.MPV_EVENT_START_FILE -> post { if (load == Load.Requested) load = Load.Started }
            MpvEvent.MPV_EVENT_FILE_LOADED -> post {
                if (load != Load.Started) return@post
                load = Load.Playing
                if (audioUrl.isNotEmpty()) mpv.command(arrayOf("audio-add", audioUrl, "select"))
                ctx.emit("load", mapOf("duration" to (mpv.getPropertyDouble("duration") ?: 0.0)))
            }
            // keep-open holds the last frame at the end, so an END_FILE of the file being played is a failure;
            // the one for the file a `loadfile replace` stops comes while the new one is still Requested.
            MpvEvent.MPV_EVENT_END_FILE -> post {
                if (load != Load.Started && load != Load.Playing) return@post
                load = Load.Idle
                ctx.emit("error", mapOf("message" to lastError.ifEmpty { "playback stopped" }))
            }
            MpvEvent.MPV_EVENT_PLAYBACK_RESTART -> post {
                if (!seeking) return@post
                seeking = false
                ctx.emit("seeked", mapOf("position" to position))
            }
        }
    }

    override fun eventProperty(property: String, value: Double) {
        when (property) {
            "time-pos" -> {
                position = value
                post { progress() }
            }
            "duration" -> duration = value
            "demuxer-cache-time" -> buffered = value
        }
    }

    override fun eventProperty(property: String, value: Boolean) {
        when (property) {
            "pause" -> post {
                paused = value
                root.keepScreenOn = !value && load == Load.Playing
                ctx.emit("pause", mapOf("paused" to value))
            }
            "paused-for-cache" -> post { ctx.emit("buffering", mapOf("buffering" to value)) }
            "eof-reached" -> if (value) post { if (load == Load.Playing) ctx.emit("ended", null) }
        }
    }

    override fun eventProperty(property: String, value: Long) {}

    override fun eventProperty(property: String, value: String) {}

    override fun eventProperty(property: String) {}

    override fun logMessage(prefix: String, level: Int, text: String) {
        if (level > MpvLogLevel.MPV_LOG_LEVEL_WARN) return
        Log.w(TAG, "[$prefix] ${text.trimEnd()}")
        if (level <= MpvLogLevel.MPV_LOG_LEVEL_ERROR) lastError = "[$prefix] ${text.trimEnd()}"
    }

    private fun progress() {
        if (load != Load.Playing) return
        val now = SystemClock.uptimeMillis()
        if (now - lastProgress < PROGRESS_EVERY_MS) return
        lastProgress = now
        ctx.emit("progress", mapOf("position" to position, "duration" to duration, "buffered" to buffered))
    }

    private fun post(block: () -> Unit) {
        main.post { if (!destroyed) block() }
    }

    private companion object {
        const val TAG = "ohg.player"
        const val PROGRESS_EVERY_MS = 250L
        val OBSERVED = listOf(
            "time-pos" to MpvFormat.MPV_FORMAT_DOUBLE,
            "duration" to MpvFormat.MPV_FORMAT_DOUBLE,
            "demuxer-cache-time" to MpvFormat.MPV_FORMAT_DOUBLE,
            "pause" to MpvFormat.MPV_FORMAT_FLAG,
            "paused-for-cache" to MpvFormat.MPV_FORMAT_FLAG,
            "eof-reached" to MpvFormat.MPV_FORMAT_FLAG,
        )
    }
}
