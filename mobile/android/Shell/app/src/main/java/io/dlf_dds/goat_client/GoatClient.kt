package io.dlf_dds.goat_client

import android.content.Context
import android.os.Build
import io.dlf_dds.goat_client.gomobile.goatclient.Client as GoClient
import io.dlf_dds.goat_client.gomobile.goatclient.Goatclient

/**
 * Process-wide holder for the gomobile-bound [GoClient] instance.
 *
 * The Go-side Client is intentionally process-singleton: it owns the
 * wg-cp0 engine context, the imported-bundle invariants, and the
 * SetAndroidProtectSocketFn callback. Multiple instances would race
 * on those; the Kotlin shell creates exactly one via [getOrCreate].
 *
 * GoatVpnService owns the long-running engine lifecycle (Run / Stop);
 * MainActivity uses this for short-lived calls (importBundle, status
 * polls). The [TunAdapterImpl] passed to NewClient is held by
 * GoatVpnService — the activity creates a "noop" adapter for its
 * brief lifetime if no service is running, which is fine because
 * importBundle and tunnelStatus do not call into the adapter.
 */
object GoatClient {

    @Volatile
    private var instance: GoClient? = null

    @Synchronized
    fun getOrCreate(ctx: Context, tunAdapter: TunAdapterImpl): GoClient {
        instance?.let { return it }
        val deviceName = "${Build.MANUFACTURER} ${Build.MODEL}".trim()
        val client = Goatclient.newClient(
            Build.VERSION.SDK_INT.toLong(),  // gomobile maps Go `int` → Java `long`
            deviceName,
            BuildConfigVersion.NAME,
            tunAdapter,                 // TunAdapter
            tunAdapter,                 // IFaceDiscover (same impl)
            tunAdapter,                 // NetworkChangeListener (same impl)
        )
        client.configure(PlatformFilesImpl(ctx.applicationContext))
        instance = client
        return client
    }

    /**
     * Returns the live engine if one exists, else creates a transient
     * client backed by a no-op adapter. Used by the activity for cheap
     * read-only calls (importBundle, tunnelStatus) that don't depend on
     * the VpnService being active.
     */
    @Synchronized
    fun getOrCreateTransient(ctx: Context): GoClient {
        instance?.let { return it }
        return getOrCreate(ctx, TunAdapterImpl.noop())
    }

    fun importBundle(ctx: Context, bytes: ByteArray) {
        getOrCreateTransient(ctx).importBundle(bytes)
    }

    fun tunnelStatus(ctx: Context): String =
        getOrCreateTransient(ctx).tunnelStatus
}

/**
 * Pulls the app version into a Kotlin object so [GoatClient] can pass
 * it to Go without re-importing BuildConfig (which lives in the app
 * module's namespace and ProGuard-strips poorly).
 */
object BuildConfigVersion {
    const val NAME = "0.0.1-pre"
}
