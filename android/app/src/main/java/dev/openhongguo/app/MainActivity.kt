package dev.openhongguo.app

import android.app.Activity
import android.os.Build
import android.os.Bundle
import android.util.Log
import android.view.View
import com.longbridge.nsh.NshView
import dev.openhongguo.core.Callback
import dev.openhongguo.core.Core
import dev.openhongguo.core.Host
import dev.openhongguo.core.HostCall
import java.io.File
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.atomic.AtomicBoolean

/**
 * The whole app is one nsh view (android/ui). It reaches the Go core through `ohg.call`, which answers
 * with the same {result} | {error} envelope as the desktop's ohg_call, and plays through `ohg.player`.
 */
class MainActivity : Activity() {
    private lateinit var view: NshView
    private lateinit var host: Host
    private val calls = ConcurrentHashMap<Long, HostCall>()
    private val players = mutableSetOf<PlayerComponent>()

    override fun onCreate(state: Bundle?) {
        super.onCreate(state)
        edgeToEdge()
        host = Core.openHost(File(filesDir, "openhongguo").path, "")
        view = NshView(this)
        view.setErrorListener { message -> Log.e(TAG, message) }
        view.handle("ohg.call") { data, reply -> startCall(data as Map<*, *>, reply::resolve) }
        view.handle("ohg.cancel") { data, _ -> calls[callId(data as Map<*, *>)]?.cancel() }
        view.registerComponent("ohg.player") { ctx -> PlayerComponent(ctx, players) }
        setContentView(view)
        val dev = BuildConfig.NSH_DEV_SERVER
        val loaded = if (dev.isNotEmpty()) view.connectDevServer(dev) else view.loadAssetBundle("nsh")
        check(loaded) { if (dev.isNotEmpty()) "cannot reach nsh dev at $dev" else "no nsh bundle in assets/nsh" }
    }

    /** `args` is the JSON argument array as text, so the Go side gets it unchanged. */
    private fun startCall(data: Map<*, *>, resolve: (String) -> Unit) {
        val id = callId(data)
        val method = data["method"] as String
        val args = data["args"] as String
        val finished = AtomicBoolean(false)
        val call = host.start(method, args, object : Callback {
            override fun done(resultJSON: String) {
                finished.set(true)
                calls.remove(id)
                resolve(resultJSON)
            }
        })
        calls[id] = call
        // A call that finished before it was stored stays out.
        if (finished.get()) calls.remove(id)
    }

    private fun callId(data: Map<*, *>) = (data["id"] as Number).toLong()

    @Suppress("DEPRECATION")
    private fun edgeToEdge() {
        if (Build.VERSION.SDK_INT >= 30) {
            window.setDecorFitsSystemWindows(false)
        } else {
            window.decorView.systemUiVisibility = View.SYSTEM_UI_FLAG_LAYOUT_STABLE or
                View.SYSTEM_UI_FLAG_LAYOUT_FULLSCREEN or
                View.SYSTEM_UI_FLAG_LAYOUT_HIDE_NAVIGATION
        }
    }

    override fun onStop() {
        super.onStop()
        players.forEach { it.pauseForBackground() }
    }

    /** Android 12 and older; newer versions reach the nsh view's own back callback. */
    @Deprecated("Deprecated in Java")
    override fun onBackPressed() {
        if (!view.handleBack()) super.onBackPressed()
    }

    override fun onDestroy() {
        view.destroy()
        calls.values.forEach { it.cancel() }
        host.close()
        super.onDestroy()
    }

    private companion object {
        const val TAG = "openhongguo"
    }
}
