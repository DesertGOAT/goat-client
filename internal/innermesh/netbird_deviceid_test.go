package innermesh

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/dlf-dds/goat-client/internal/innermesh/fakemgmt"
	"github.com/dlf-dds/goat-client/internal/innermesh/fakesignal"
)

// TestNetbird_PeerNameIsComposed is the hermetic equivalent of the
// PR #53 test-plan's "register against a netbird-fork mgmt-server,
// confirm peer name in mgmt UI reads 'ops-04 (<hostname>)'" manual
// smoke. The mgmt UI renders peer.Meta.Hostname; that field comes
// from the LoginRequest the client sends on the wire. Driving the
// real *Netbird against fakemgmt+fakesignal and reading
// Server.LastLogin().GetMeta().GetHostname() observes the same value
// the mgmt UI would render — so passing this in CI closes the
// test-plan item without needing a live netbird-fork mgmt-server.
//
// The path under test:
//
//	Config.BundleDeviceID = "ops-04"
//	NewNetbird("test-host")               // device-reported half
//	Netbird.Connect → composeIdentity     // "ops-04 (test-host)"
//	→ embed.Options.DeviceName
//	→ ctx[system.DeviceNameCtxKey]
//	→ system.Info.Hostname
//	→ infoToMetaData(sysInfo).Hostname
//	→ LoginRequest.Meta.Hostname          // assert here
func TestNetbird_PeerNameIsComposed(t *testing.T) {
	// Same upstream-netbird embed.Client race as
	// TestNetbird_LifecycleAgainstFakes — skip under -race.
	if raceDetectorEnabled {
		t.Skip("upstream netbird embed.Client connect/stop race; tracked separately")
	}

	sig, err := fakesignal.Listen(t)
	if err != nil {
		t.Fatalf("fakesignal.Listen: %v", err)
	}
	mgmt, err := fakemgmt.Listen(t, fakemgmt.WithSignalURI(sig.Addr()))
	if err != nil {
		t.Fatalf("fakemgmt.Listen: %v", err)
	}

	const (
		bundleDeviceID = "ops-04"
		deviceReported = "test-host"
		wantHostname   = "ops-04 (test-host)"
	)

	nb := NewNetbird(deviceReported)
	t.Cleanup(func() { _ = nb.Close() })

	cfg := Config{
		ManagementURL:  "http://" + mgmt.Addr(),
		SetupKey:       "11111111-1111-1111-1111-111111111111",
		BundleDeviceID: bundleDeviceID,
	}
	if err := nb.Configure(cfg); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := nb.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v\nlogs:\n%s", err, strings.Join(nb.Logs(50), "\n"))
	}
	t.Cleanup(func() { _ = nb.Disconnect(context.Background()) })

	// Login fires synchronously inside Connect for the embed client,
	// so LastLogin() is populated by the time Connect returns. Read
	// it once; no polling needed.
	got := mgmt.LastLogin()
	if got == nil {
		t.Fatalf("fakemgmt.LastLogin() returned nil after successful Connect\nlogs:\n%s",
			strings.Join(nb.Logs(50), "\n"))
	}
	meta := got.GetMeta()
	if meta == nil {
		t.Fatalf("LoginRequest.Meta is nil — embed.Client should always populate sysInfo\nlogs:\n%s",
			strings.Join(nb.Logs(50), "\n"))
	}
	if meta.GetHostname() != wantHostname {
		t.Errorf("LoginRequest.Meta.Hostname:\n  got:  %q\n  want: %q\n(this is the value netbird's mgmt UI renders as peer.Meta.Hostname,\nso a mismatch means the operator would see the wrong name on the wire)",
			meta.GetHostname(), wantHostname)
	}
}
