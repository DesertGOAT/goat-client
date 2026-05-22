package innermesh

import "testing"

// TestComposeIdentity covers the matching workflow's wire-side contract:
// what string the operator sees as peer.Meta.Hostname (the {hostname}
// substitution in dfarrel1/netbird c33517a5's AutoPeerNameTemplate) for
// each combination of bundle and device halves. Format changes here are
// observable in the netbird mgmt UI, so this test is the canary.
func TestComposeIdentity(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name           string
		bundleDeviceID string
		deviceID       string
		want           string
	}{
		{"both present", "ops-laptop-04", "Dene's iPhone", "ops-laptop-04 (Dene's iPhone)"},
		{"bundle only", "ops-laptop-04", "", "ops-laptop-04"},
		{"device only", "", "Dene's iPhone", "Dene's iPhone"},
		{"both empty", "", "", ""},
		{"trims surrounding whitespace", "  ops-04  ", "  iPhone 17  ", "ops-04 (iPhone 17)"},
		{"whitespace-only is treated as empty", "   ", "Pixel 8", "Pixel 8"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := composeIdentity(tc.bundleDeviceID, tc.deviceID)
			if got != tc.want {
				t.Errorf("composeIdentity(%q, %q) = %q; want %q",
					tc.bundleDeviceID, tc.deviceID, got, tc.want)
			}
		})
	}
}
