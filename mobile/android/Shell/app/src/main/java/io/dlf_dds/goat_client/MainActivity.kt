package io.dlf_dds.goat_client

import android.content.Intent
import android.net.VpnService
import android.os.Bundle
import android.util.Log
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AppCompatActivity
import androidx.lifecycle.lifecycleScope
import io.dlf_dds.goat_client.databinding.ActivityMainBinding
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.collectLatest
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import org.json.JSONObject

/**
 * MainActivity is the user-facing entry point of the goat-client Android shell.
 *
 * Three flows live here, all wired against [GoatClient] (the Kotlin-side
 * holder for the gomobile-bound goatclient.Client and its singleton lifecycle):
 *
 *   1. Import-bundle: SAF document picker → bytes → GoatClient.importBundle().
 *      Once Track A's internal/bundle parser lands, the SDK will validate the
 *      Ed25519 signature against the pinned offline-CA root and reject on failure.
 *
 *   2. Prepare + start VpnService: VpnService.prepare() asks the system for the
 *      always-on-VPN consent dialog (one-time per app), then the resulting
 *      ActivityResult triggers GoatVpnService.start().
 *
 *   3. Status poll: a background coroutine polls GoatClient.tunnelStatus()
 *      every 1s and reflects state to the UI. Cheap (in-memory in Go); the
 *      streaming RPC for handshake / bytes-in-out arrives with Track A.
 *
 * QR-code bundle import (CameraX + ZXing) is deferred to a follow-up — see
 * mobile/android/README.md "Open follow-ups".
 */
class MainActivity : AppCompatActivity() {

    private lateinit var binding: ActivityMainBinding

    private val statusFlow = MutableStateFlow(StatusSnapshot.unconfigured())

    private val pickBundle = registerForActivityResult(
        ActivityResultContracts.OpenDocument()
    ) { uri ->
        if (uri == null) return@registerForActivityResult
        lifecycleScope.launch {
            val bytes = withContext(Dispatchers.IO) {
                contentResolver.openInputStream(uri)?.use { it.readBytes() }
            }
            if (bytes == null || bytes.isEmpty()) {
                Log.w(TAG, "bundle import: empty payload from $uri")
                return@launch
            }
            try {
                GoatClient.importBundle(applicationContext, bytes)
                refreshStatus()
            } catch (t: Throwable) {
                Log.e(TAG, "bundle import failed", t)
            }
        }
    }

    private val prepareVpn = registerForActivityResult(
        ActivityResultContracts.StartActivityForResult()
    ) { result ->
        if (result.resultCode == RESULT_OK) {
            startVpnService()
        } else {
            Log.w(TAG, "VpnService.prepare() denied by user")
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        binding = ActivityMainBinding.inflate(layoutInflater)
        setContentView(binding.root)

        binding.importBundleButton.setOnClickListener {
            // application/octet-stream covers the .cbor bundle MIME shape;
            // some pickers also surface */* — we accept both.
            pickBundle.launch(arrayOf("application/octet-stream", "application/cbor", "*/*"))
        }

        binding.connectButton.setOnClickListener {
            val state = statusFlow.value.state
            if (state == "connected" || state == "connecting") {
                stopVpnService()
            } else {
                val prepareIntent = VpnService.prepare(this)
                if (prepareIntent != null) {
                    prepareVpn.launch(prepareIntent)
                } else {
                    startVpnService()
                }
            }
        }

        lifecycleScope.launch {
            statusFlow.collectLatest { snap -> renderStatus(snap) }
        }

        // Initial reflection of "do we already have a bundle persisted?"
        refreshStatus()
        // Cheap 1Hz poll while activity is in the foreground.
        lifecycleScope.launch {
            while (true) {
                kotlinx.coroutines.delay(1000)
                refreshStatus()
            }
        }
    }

    private fun startVpnService() {
        val intent = Intent(this, GoatVpnService::class.java).apply {
            action = GoatVpnService.ACTION_START
        }
        startForegroundService(intent)
    }

    private fun stopVpnService() {
        val intent = Intent(this, GoatVpnService::class.java).apply {
            action = GoatVpnService.ACTION_STOP
        }
        startService(intent)
    }

    private fun refreshStatus() {
        statusFlow.value = StatusSnapshot.parse(GoatClient.tunnelStatus(applicationContext))
    }

    private fun renderStatus(snap: StatusSnapshot) {
        binding.statusText.text = when (snap.state) {
            "unconfigured" -> getString(R.string.status_unconfigured)
            "imported"     -> getString(R.string.status_imported)
            "connecting"   -> getString(R.string.status_connecting)
            "connected"    -> getString(R.string.status_connected)
            "disconnected" -> getString(R.string.status_disconnected)
            "error"        -> getString(R.string.status_error)
            else           -> snap.state
        }
        binding.detailText.text = listOfNotNull(
            snap.bundleSum.takeIf { it.isNotEmpty() }?.let { "bundle ${it.take(12)}…" },
            snap.reason.takeIf { it.isNotEmpty() }
        ).joinToString(" · ")

        binding.connectButton.isEnabled = snap.state != "unconfigured"
        binding.connectButton.text = when (snap.state) {
            "connected", "connecting" -> getString(R.string.disconnect)
            else                      -> getString(R.string.connect)
        }
    }

    companion object {
        private const val TAG = "GoatMainActivity"
    }
}

/**
 * Decoded shape of [io.dlf_dds.goat_client.gomobile.goatclient.Client.getTunnelStatus]'s
 * JSON payload. The Go side guarantees this shape; UI tolerates missing optional fields.
 */
data class StatusSnapshot(
    val state: String,
    val reason: String,
    val since: String,
    val bundleSum: String,
    val deviceName: String,
) {
    companion object {
        fun unconfigured() = StatusSnapshot("unconfigured", "", "", "", "")
        fun parse(json: String): StatusSnapshot {
            return try {
                val o = JSONObject(json)
                StatusSnapshot(
                    state      = o.optString("state", "unconfigured"),
                    reason     = o.optString("reason", ""),
                    since      = o.optString("since", ""),
                    bundleSum  = o.optString("bundleSum", ""),
                    deviceName = o.optString("deviceName", ""),
                )
            } catch (_: Throwable) {
                unconfigured()
            }
        }
    }
}
