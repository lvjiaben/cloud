package merchantcontext

import (
	"context"
	"encoding/json"
	"testing"

	"lina-core/pkg/plugin/capability/capregistry"
	"lina-plugin-idc-core/backend/cap/idccap"
)

type capabilityServiceStub struct {
	calls int
	value idccap.MerchantContext
}

func (s *capabilityServiceStub) CurrentMerchant(context.Context) (idccap.MerchantContext, error) {
	s.calls++
	return s.value, nil
}

func TestCapabilityInvokerFailsClosedAndReturnsValidatedProjection(t *testing.T) {
	t.Parallel()
	service := &capabilityServiceStub{value: idccap.MerchantContext{
		TenantID: 0, MerchantNo: "merchant_local", DisplayName: "Local merchant",
		DefaultLocale: "zh-CN", DefaultCurrency: "CNY", Timezone: "Asia/Shanghai",
		Enabled: true, Version: 1,
	}}
	invoker := NewCapabilityInvoker()
	if _, err := invoker.Invoke(context.Background(), capregistry.Invocation{}); err == nil {
		t.Fatal("unbound invoker accepted traffic")
	}
	if err := invoker.Bind(service); err != nil {
		t.Fatalf("bind invoker: %v", err)
	}
	if err := invoker.Bind(service); err == nil {
		t.Fatal("invoker accepted a second binding")
	}
	request := capregistry.Invocation{
		CallerPluginID: "dynamic-consumer", OwnerPluginID: idccap.OwnerPluginID,
		Service: idccap.ServiceName, Version: string(idccap.ContractVersion),
		Method: string(idccap.MethodMerchantContextCurrent),
	}
	result, err := invoker.Invoke(context.Background(), request)
	if err != nil || result == nil || result.Status != 0 || service.calls != 1 {
		t.Fatalf("invoke merchant context: result=%+v calls=%d err=%v", result, service.calls, err)
	}
	var decoded idccap.MerchantContext
	if err = json.Unmarshal(result.Payload, &decoded); err != nil || decoded.MerchantNo != service.value.MerchantNo {
		t.Fatalf("decode merchant context: decoded=%+v err=%v", decoded, err)
	}
	invalid := request
	invalid.ResourceRef = "escape"
	if _, err = invoker.Invoke(context.Background(), invalid); err == nil || service.calls != 1 {
		t.Fatalf("invalid envelope reached owner: calls=%d err=%v", service.calls, err)
	}
}
