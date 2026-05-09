//go:build ios

package GoatClientSDK

// NetworkChangeListener is implemented on the Swift side; the Go runtime
// invokes OnNetworkChange whenever the host network situation shifts (e.g.
// the NEPacketTunnelProvider receives a path-monitor update). Mirrors
// netbird's listener.NetworkChangeListener narrowly — we only need the one
// callback shape here.
type NetworkChangeListener interface {
	// OnNetworkChange is fired when the connectivity state changes.
	// Implementations should be cheap; long work belongs on a background
	// queue Swift-side.
	OnNetworkChange()
}

// DnsManager is implemented on the Swift side and bridges to NEDNSSettings on
// the NEPacketTunnelProvider. ApplyDns receives a JSON-encoded DNS config
// (matching the host_ios.go contract Track A is forking) so we don't have to
// describe a complex struct across the gomobile boundary.
type DnsManager interface {
	// ApplyDns is called with a JSON string describing the desired DNS
	// configuration for the tunnel. The Swift side decodes and applies it
	// via NEDNSSettings on the NEPacketTunnelNetworkSettings object.
	ApplyDns(jsonConfig string)
}

// CustomLogger is implemented on the Swift side. The Go logger forwards
// records into it so the unified iOS log (`os_log`) gets formatted output.
// Optional: if Swift passes nil, logs go to stderr only.
type CustomLogger interface {
	Debug(message string)
	Info(message string)
	Error(message string)
}
