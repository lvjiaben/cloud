package domainhostcall

import (
	"context"
	"encoding/json"
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-core/pkg/plugin/pluginbridge/protocol"
)

func TestPluginBusinessExtensionGuestMethodsUseGovernedJSONCalls(t *testing.T) {
	calls := make([]string, 0, 3)
	client := PluginRegistry(func(service, method string, request []byte, out any) error {
		if service != protocol.HostServicePlugins {
			t.Fatalf("unexpected service %s", service)
		}
		envelope, err := protocol.UnmarshalHostServiceJSONRequest(request)
		if err != nil {
			return err
		}
		if len(envelope.Value) == 0 {
			t.Fatal("expected encoded JSON input")
		}
		calls = append(calls, method)
		var value any
		switch method {
		case protocol.HostServiceMethodPluginsBusinessExtensionsList:
			value = &capmodel.PageResult[*capmodel.BusinessExtensionProviderEvidence]{Items: []*capmodel.BusinessExtensionProviderEvidence{{PluginID: "ssl-provider"}}, Total: 1}
		case protocol.HostServiceMethodPluginsBusinessExtensionsExact:
			value = &capmodel.BusinessExtensionProviderEvidence{PluginID: "ssl-provider", ReleaseDigest: "release"}
		case protocol.HostServiceMethodPluginsBusinessExtensionsExactBatch:
			value = []*capmodel.BusinessExtensionProviderEvidence{{PluginID: "ssl-provider"}, {PluginID: "phone-provider"}}
		default:
			t.Fatalf("unexpected method %s", method)
		}
		encoded, _ := json.Marshal(value)
		return json.Unmarshal(encoded, out)
	})
	page, err := client.ListBusinessExtensionProviders(context.Background(), plugincap.BusinessExtensionProviderListInput{Point: "product-fulfillment"})
	if err != nil || page.Total != 1 {
		t.Fatalf("list providers: page=%#v err=%v", page, err)
	}
	exact, err := client.GetBusinessExtensionProviderExact(context.Background(), plugincap.BusinessExtensionProviderExactInput{PluginID: "ssl-provider", ReleaseVersion: "v1", Point: "product-fulfillment", ContractVersion: "v1", ReleaseDigest: "release", DeclarationDigest: "declaration"})
	if err != nil || exact.PluginID != "ssl-provider" {
		t.Fatalf("exact provider: evidence=%#v err=%v", exact, err)
	}
	batch, err := client.GetBusinessExtensionProvidersExact(context.Background(), []plugincap.BusinessExtensionProviderExactInput{{PluginID: "ssl-provider"}, {PluginID: "phone-provider"}})
	if err != nil || len(batch) != 2 || batch[1].PluginID != "phone-provider" {
		t.Fatalf("batch exact providers: evidence=%#v err=%v", batch, err)
	}
	if len(calls) != 3 || calls[0] != protocol.HostServiceMethodPluginsBusinessExtensionsList || calls[1] != protocol.HostServiceMethodPluginsBusinessExtensionsExact || calls[2] != protocol.HostServiceMethodPluginsBusinessExtensionsExactBatch {
		t.Fatalf("unexpected calls: %#v", calls)
	}
}
