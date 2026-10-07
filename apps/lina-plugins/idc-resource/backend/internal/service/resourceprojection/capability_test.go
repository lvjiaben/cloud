package resourceprojection

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"lina-core/pkg/plugin/capability/capregistry"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

type directoryStub struct{}

func (directoryStub) ListCapabilities(_ context.Context, routes []serveroperationresourcecap.CapabilityRoute) ([]serveroperationresourcecap.CapabilityProjection, error) {
	return []serveroperationresourcecap.CapabilityProjection{{ServiceNo: routes[0].ServiceNo, Actions: []serveroperationresourcecap.Action{serveroperationresourcecap.ActionStop, serveroperationresourcecap.ActionStart}, ObservedAt: time.Now().UTC()}}, nil
}

type currentCustomerServersStub struct{}

func (currentCustomerServersStub) ListCurrentCustomerServers(_ context.Context, request resourcecap.CurrentCustomerServerListRequest) (*resourcecap.CurrentCustomerServerPage, error) {
	return &resourcecap.CurrentCustomerServerPage{Items: []resourcecap.CurrentCustomerServerItem{testCurrentCustomerServer()}}, nil
}

func (currentCustomerServersStub) GetCurrentCustomerServer(_ context.Context, request resourcecap.CurrentCustomerServerGetRequest) (*resourcecap.CurrentCustomerServerResult, error) {
	return &resourcecap.CurrentCustomerServerResult{Item: testCurrentCustomerServer()}, nil
}

func testCurrentCustomerServer() resourcecap.CurrentCustomerServerItem {
	return resourcecap.CurrentCustomerServerItem{
		ServerNo:        "SVR-0123456789ABCDEF0123456789ABCDEF",
		ProductNameZhCN: "云服务器", ProductNameEnUS: "Cloud server",
		ChannelName: "生产资源", PoolName: "华东资源池", Kind: "cloud_server",
		State: "ready", LatestOperation: "idle", BoundAt: 1, UpdatedAt: 1,
	}
}

func TestCapabilityInvokerBindsOnceAndRejectsWrongEnvelope(t *testing.T) {
	service, err := New(directoryStub{})
	if err != nil {
		t.Fatal(err)
	}
	invoker := NewCapabilityInvoker()
	servers := currentCustomerServersStub{}
	if err = invoker.Bind(service, servers); err != nil {
		t.Fatal(err)
	}
	if invoker.Bind(service, servers) == nil {
		t.Fatal("expected duplicate binding to fail")
	}
	request := resourcecap.ServerCapabilitiesRequest{Routes: []resourcecap.ServerCapabilityRoute{{ServiceNo: "svc-1", ChannelNo: "channel-1", PoolNo: "pool-1"}}}
	payload, _ := json.Marshal(request)
	invocation := capregistry.Invocation{CallerPluginID: "idc-ai", OwnerPluginID: resourcecap.OwnerPluginID, Service: resourcecap.ServiceName, Version: string(resourcecap.ReadVersion), Method: string(resourcecap.MethodServerCapabilitiesList), Payload: payload}
	result, err := invoker.Invoke(context.Background(), invocation)
	if err != nil || result == nil || len(result.Payload) == 0 {
		t.Fatalf("expected valid invocation: %#v err=%v", result, err)
	}
	invocation.Table = "plugin_idc_resource_mapping"
	if _, err = invoker.Invoke(context.Background(), invocation); err == nil {
		t.Fatal("expected table escape to fail closed")
	}
}

func TestCapabilityInvokerAuthorizesCurrentCustomerMethodsIndependently(t *testing.T) {
	service, err := New(directoryStub{})
	if err != nil {
		t.Fatal(err)
	}
	invoker := NewCapabilityInvoker()
	if err = invoker.Bind(service, currentCustomerServersStub{}); err != nil {
		t.Fatal(err)
	}
	serverNo := "SVR-0123456789ABCDEF0123456789ABCDEF"
	tests := []struct {
		method  resourcecap.Method
		request any
	}{
		{method: resourcecap.MethodCurrentCustomerServersList, request: resourcecap.CurrentCustomerServerListRequest{Limit: 20}},
		{method: resourcecap.MethodCurrentCustomerServerGet, request: resourcecap.CurrentCustomerServerGetRequest{ServerNo: serverNo}},
	}
	for _, test := range tests {
		payload, marshalErr := json.Marshal(test.request)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		result, invokeErr := invoker.Invoke(context.Background(), capregistry.Invocation{
			CallerPluginID: "idc-zjmfcloud", OwnerPluginID: resourcecap.OwnerPluginID,
			Service: resourcecap.ServiceName, Version: string(resourcecap.ReadVersion),
			Method: string(test.method), Payload: payload,
		})
		if invokeErr != nil || result == nil || len(result.Payload) == 0 {
			t.Fatalf("invoke %s result=%+v err=%v", test.method, result, invokeErr)
		}
	}
}
