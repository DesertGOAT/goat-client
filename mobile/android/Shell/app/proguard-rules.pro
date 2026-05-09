# Keep the gomobile-bound classes — they are reflected by the
# Go-side bridge and by Java method-name lookups in the AAR runtime.
-keep class io.dlf_dds.goat_client.gomobile.** { *; }
-keep class go.** { *; }

# Keep our Kotlin VpnService — Android instantiates it by name from
# the manifest entry, ProGuard can't see the call site.
-keep class io.dlf_dds.goat_client.GoatVpnService { *; }
