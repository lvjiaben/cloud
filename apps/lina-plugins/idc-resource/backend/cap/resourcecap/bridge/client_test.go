package bridge

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

func TestClientUsesExactOwnerEnvelopeAndValidatesResponse(t *testing.T) {
	request := resourcecap.ServerCapabilitiesRequest{Routes: []resourcecap.ServerCapabilityRoute{{ServiceNo: "svc-1", ChannelNo: "channel-1", PoolNo: "pool-1"}}}
	client, err := NewClient(func(owner, service, version, method, resourceRef, table string, payload []byte) ([]byte, error) {
		if owner != resourcecap.OwnerPluginID || service != resourcecap.ServiceName || version != string(resourcecap.ReadVersion) ||
			method != string(resourcecap.MethodServerCapabilitiesList) || resourceRef != "" || table != "" {
			t.Fatalf("unexpected envelope: %q %q %q %q %q %q", owner, service, version, method, resourceRef, table)
		}
		var decoded resourcecap.ServerCapabilitiesRequest
		if json.Unmarshal(payload, &decoded) != nil || decoded.Routes[0] != request.Routes[0] {
			t.Fatalf("unexpected request: %#v", decoded)
		}
		return json.Marshal(resourcecap.ServerCapabilitiesResult{Items: []resourcecap.ServerCapabilityProjection{{ServiceNo: "svc-1", Capabilities: []resourcecap.CapabilityCode{resourcecap.CapabilityStart}, ObservedAt: time.Now().UTC()}}})
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.ListServerCapabilities(context.Background(), request)
	if err != nil || len(result.Items) != 1 || result.Items[0].Capabilities[0] != resourcecap.CapabilityStart {
		t.Fatalf("unexpected result: %#v err=%v", result, err)
	}
}

func TestClientRejectsSecretShapedOrMismatchedPayloads(t *testing.T) {
	client, err := NewClient(func(_, _, _, _, _, _ string, _ []byte) ([]byte, error) {
		return []byte(`{"items":[{"serviceNo":"another-service","capabilities":[]}]}`), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	request := resourcecap.ServerCapabilitiesRequest{Routes: []resourcecap.ServerCapabilityRoute{{ServiceNo: "svc-1", ChannelNo: "channel-1", PoolNo: "pool-1"}}}
	if _, err = client.ListServerCapabilities(context.Background(), request); err == nil {
		t.Fatal("expected mismatched response to fail closed")
	}
	request.Routes[0].ChannelNo = "access-token"
	if _, err = client.ListServerCapabilities(context.Background(), request); err == nil {
		t.Fatal("expected credential-shaped route to fail closed")
	}
}

func TestClientInvokesCurrentCustomerMethodsWithoutCustomerOrActionReference(t *testing.T) {
	serverNo := "SVR-0123456789ABCDEF0123456789ABCDEF"
	item := resourcecap.CurrentCustomerServerItem{
		ServerNo: serverNo, ProductNameZhCN: "云服务器", ProductNameEnUS: "Cloud server",
		ChannelName: "生产资源", PoolName: "华东资源池", Kind: "cloud_server",
		State: "ready", LatestOperation: "idle",
		AvailableActions: []resourcecap.ServerActionCode{resourcecap.ServerActionStart},
		BoundAt:          1, UpdatedAt: 1,
	}
	client, err := NewClient(func(owner, service, version, method, resourceRef, table string, payload []byte) ([]byte, error) {
		if owner != resourcecap.OwnerPluginID || service != resourcecap.ServiceName ||
			version != string(resourcecap.ReadVersion) || resourceRef != "" || table != "" {
			t.Fatalf("unexpected owner envelope: %q %q %q %q %q", owner, service, version, resourceRef, table)
		}
		if string(payload) == "" || string(payload) == "{}" {
			t.Fatal("expected one bounded request payload")
		}
		switch resourcecap.Method(method) {
		case resourcecap.MethodCurrentCustomerServersList:
			return json.Marshal(resourcecap.CurrentCustomerServerPage{Items: []resourcecap.CurrentCustomerServerItem{item}})
		case resourcecap.MethodCurrentCustomerServerGet:
			return json.Marshal(resourcecap.CurrentCustomerServerResult{Item: item})
		default:
			t.Fatalf("unexpected method %q", method)
			return nil, nil
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	page, err := client.ListCurrentCustomerServers(context.Background(), resourcecap.CurrentCustomerServerListRequest{Limit: 20})
	if err != nil || len(page.Items) != 1 || page.Items[0].ServerNo != serverNo {
		t.Fatalf("unexpected customer page=%+v err=%v", page, err)
	}
	result, err := client.GetCurrentCustomerServer(context.Background(), resourcecap.CurrentCustomerServerGetRequest{ServerNo: serverNo})
	if err != nil || result.Item.ServerNo != serverNo {
		t.Fatalf("unexpected customer detail=%+v err=%v", result, err)
	}
}
