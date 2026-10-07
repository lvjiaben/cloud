package resourcecap

import (
	"testing"
	"time"
)

func TestAuthenticationSessionContractsRejectUnsafeScopeAndGeneration(t *testing.T) {
	t.Parallel()
	if err := (AuthenticationSessionRequest{ChannelRef: "channel-safe", TTL: time.Minute}).Validate(); err != nil {
		t.Fatalf("valid authentication session request rejected: %v", err)
	}
	for _, request := range []AuthenticationSessionRequest{
		{ChannelRef: " token=must-not-pass", TTL: time.Minute},
		{ChannelRef: "channel-safe", TTL: time.Second},
		{ChannelRef: "channel-safe", TTL: 2 * time.Hour},
	} {
		if err := request.Validate(); err == nil {
			t.Fatalf("unsafe authentication session request accepted: %#v", request)
		}
	}
	if err := (AuthenticationSessionInvalidation{ChannelRef: "channel-safe", Generation: 1}).Validate(); err != nil {
		t.Fatalf("valid authentication invalidation rejected: %v", err)
	}
	if err := (AuthenticationSessionInvalidation{ChannelRef: "channel-safe"}).Validate(); err == nil {
		t.Fatal("zero authentication generation was accepted")
	}
}
