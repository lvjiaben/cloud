// This file proves owned dynamic form routing with real resource projections.
package customeroperations

import (
	"context"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/accessorydirectory"
	"strings"
	"testing"
	"time"
)

type configurationOwner struct {
	servercommercialcap.Service
	read  servercommercialcap.ConfigurationContextRequest
	quote servercommercialcap.StructuredTargetInput
	calls int
	wait  bool
}

func (o *configurationOwner) ReadConfigurationContext(ctx context.Context, input servercommercialcap.ConfigurationContextRequest) (*servercommercialcap.ConfigurationContext, error) {
	o.calls++
	o.read = input
	if o.wait {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return &servercommercialcap.ConfigurationContext{EntitlementNo: input.Target.EntitlementNo, ConfigurationRevision: "revision", Values: input.Values}, nil
}
func (o *configurationOwner) QuoteTarget(_ context.Context, input servercommercialcap.StructuredTargetInput) (*servercommercialcap.TargetQuote, error) {
	o.calls++
	o.quote = input
	return &servercommercialcap.TargetQuote{QuoteNo: "frozen-target"}, nil
}
func TestNativeConfigurationOwnedTargetPostgreSQL(t *testing.T) {
	f := newManagementFixture(t)
	directory, err := accessorydirectory.New(f.db)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if _, err = directory.Publish(f.ctx, accessorydirectory.PublishInput{MappingNo: "mapping-1", LocalResourceNo: "SRV_" + strings.Repeat("1", 32), ChannelNo: "channel-main", ExternalResourceRef: "external-1", Snapshot: resourcecap.AccessorySnapshot{ObservedAt: now, ExpiresAt: now.Add(time.Minute), Items: []resourcecap.AccessoryItem{}}}); err != nil {
		t.Fatal(err)
	}
	owner := &configurationOwner{}
	service, err := NewCommercial(f.service, owner)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Configuration(f.ctx, ConfigurationCommand{ServerNo: f.serverNo}); err != nil {
		t.Fatal(err)
	}
	if owner.read.Values != nil || owner.read.Target.LocalResourceNo != "SRV_"+strings.Repeat("1", 32) || owner.read.Target.CustomerNo != "customer-a" {
		t.Fatal("current form target not source owned")
	}
	command := ConfigurationCommand{ServerNo: f.serverNo, ConfigurationRevision: "revision", BillingTermToken: "term", Values: []servercommercialcap.TypedPurchaseValue{}}
	if _, err = service.Configuration(f.ctx, command); err != nil || owner.read.Values == nil {
		t.Fatal("empty edit confused with current read", err)
	}
	result, err := service.QuoteTarget(f.ctx, command)
	if err != nil || result.QuoteNo != "frozen-target" || owner.quote.BillingTermToken != "term" {
		t.Fatal("target not delegated", err)
	}
	owner.wait = true
	started := time.Now()
	if _, err = service.Configuration(f.ctx, command); err != ErrConfigurationTimeout {
		t.Fatal("timeout not surfaced", err)
	}
	if time.Since(started) > 3*time.Second {
		t.Fatal("dynamic form exceeded total loading budget")
	}
	owner.wait = false
	before, providerCalls := owner.calls, f.provider.calls
	for _, bad := range []ConfigurationCommand{{ServerNo: f.serverNo, Values: []servercommercialcap.TypedPurchaseValue{}}, {ServerNo: f.serverNo, ConfigurationRevision: "revision", Values: make([]servercommercialcap.TypedPurchaseValue, 101)}} {
		if _, err = service.Configuration(f.ctx, bad); err == nil {
			t.Fatal("invalid input accepted")
		}
	}
	command.ServerNo = "SVR-" + strings.Repeat("F", 32)
	if _, err = service.Configuration(f.ctx, command); err == nil {
		t.Fatal("foreign target accepted")
	}
	if owner.calls != before || f.provider.calls != providerCalls {
		t.Fatal("unauthorized/invalid form reached compiler or upstream")
	}
	f.customers.customer = "customer-b"
	command.ServerNo = f.serverNo
	if _, err = service.QuoteTarget(f.ctx, command); err == nil {
		t.Fatal("ownership change accepted")
	}
	if owner.calls != before || f.provider.calls != providerCalls {
		t.Fatal("foreign ownership observed upstream")
	}
}
