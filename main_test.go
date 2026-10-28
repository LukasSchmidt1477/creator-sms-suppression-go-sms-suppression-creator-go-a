package main

import "testing"

func TestDecideSuppressesOptedOutSubscriber(t *testing.T) {
	tests := []struct {
		name     string
		optedOut bool
		status   string
		reason   string
	}{
		{"active subscriber queues asset message", false, "queued", ""},
		{"opted out subscriber is suppressed", true, "suppressed", "subscriber opted out"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decide(Subscriber{Phone: "+15550001", OptedOut: tt.optedOut, AssetID: "asset-42"})
			if got.Status != tt.status || got.Reason != tt.reason {
				t.Fatalf("decision = %#v, want status %q reason %q", got, tt.status, tt.reason)
			}
		})
	}
}
