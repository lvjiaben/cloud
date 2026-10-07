package backend

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"lina-core/pkg/plugin/pluginbridge/protocol"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

type pageServerServiceStub struct {
	item resourcecap.CurrentCustomerServerItem
}

func (stub pageServerServiceStub) ListCurrentCustomerServers(_ context.Context, _ resourcecap.CurrentCustomerServerListRequest) (*resourcecap.CurrentCustomerServerPage, error) {
	return &resourcecap.CurrentCustomerServerPage{Items: []resourcecap.CurrentCustomerServerItem{stub.item}}, nil
}

func (stub pageServerServiceStub) GetCurrentCustomerServer(_ context.Context, _ resourcecap.CurrentCustomerServerGetRequest) (*resourcecap.CurrentCustomerServerResult, error) {
	return &resourcecap.CurrentCustomerServerResult{Item: stub.item}, nil
}

func TestCustomerPageBridgeReturnsOnlyCustomerSafeProjection(t *testing.T) {
	item := resourcecap.CurrentCustomerServerItem{
		ServerNo: "SVR-0123456789ABCDEF0123456789ABCDEF", ProductNameZhCN: "商务云", ProductNameEnUS: "Business Cloud",
		ChannelName: "华东渠道", PoolName: "上海资源池", Kind: "cloud_server", State: "ready", LatestOperation: "idle",
		BoundAt: 1777000000000, UpdatedAt: 1777000100000,
	}
	handler := NewGuestHandler(&fakeConfidentialOutbound{}, pageServerServiceStub{item: item})
	request := &protocol.BridgeRequestEnvelopeV1{
		PluginID: pluginID,
		Route:    &protocol.RouteMatchSnapshotV1{RequestType: "ServerListReq", Method: "GET", QueryValues: map[string][]string{"limit": {"20"}}},
		Request:  &protocol.HTTPRequestSnapshotV1{Method: "GET"},
	}
	response, err := handler(request)
	if err != nil || response.StatusCode != 200 {
		t.Fatalf("unexpected page bridge response: response=%#v err=%v", response, err)
	}
	var decoded struct {
		Items []map[string]any `json:"items"`
	}
	if err = json.Unmarshal(response.Body, &decoded); err != nil || len(decoded.Items) != 1 {
		t.Fatalf("decode safe page response: %v body=%s", err, response.Body)
	}
	for _, forbidden := range []string{"actionReference", "entitlementNo", "mappingNo", "taskNo", "providerNo", "customerNo"} {
		if strings.Contains(string(response.Body), forbidden) {
			t.Fatalf("page bridge exposed forbidden field %s: %s", forbidden, response.Body)
		}
	}
}
