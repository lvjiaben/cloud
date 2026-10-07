package productdeliverydirectory

import (
	"context"
	"encoding/json"
	"testing"

	"lina-core/pkg/plugin/capability/capregistry"
	"lina-plugin-idc-core/backend/cap/productdeliverycap"
)

type deliveryServiceStub struct{ caller string }

func (s *deliveryServiceStub) ListCurrentCustomerDeliveries(_ context.Context, caller string, _ productdeliverycap.CurrentCustomerDeliveryListRequest) (*productdeliverycap.CurrentCustomerDeliveryPage, error) {
	s.caller = caller
	return &productdeliverycap.CurrentCustomerDeliveryPage{Items: []productdeliverycap.CurrentCustomerDeliveryItem{{
		DeliveryNo: "DLV-0123456789ABCDEF0123456789ABCDEF", ProductNameZhCN: "商务手机",
		ProductNameEnUS: "Business phone", Status: "processing", Quantity: 1, UpdatedAt: 1,
	}}}, nil
}

func (s *deliveryServiceStub) GetCurrentCustomerDelivery(_ context.Context, caller string, request productdeliverycap.CurrentCustomerDeliveryGetRequest) (*productdeliverycap.CurrentCustomerDeliveryResult, error) {
	s.caller = caller
	return &productdeliverycap.CurrentCustomerDeliveryResult{Item: productdeliverycap.CurrentCustomerDeliveryItem{
		DeliveryNo: request.DeliveryNo, ProductNameZhCN: "商务手机", ProductNameEnUS: "Business phone",
		Status: "processing", Quantity: 1, UpdatedAt: 1,
	}}, nil
}

func TestCapabilityInvokerUsesTrustedCallerPluginIdentity(t *testing.T) {
	service := &deliveryServiceStub{}
	invoker := NewCapabilityInvoker()
	if err := invoker.Bind(service); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(productdeliverycap.CurrentCustomerDeliveryListRequest{Limit: 20})
	result, err := invoker.Invoke(context.Background(), capregistry.Invocation{
		CallerPluginID: "phone-fulfillment", OwnerPluginID: productdeliverycap.OwnerPluginID,
		Service: productdeliverycap.ServiceName, Version: productdeliverycap.ContractVersion,
		Method: productdeliverycap.MethodCurrentCustomerDeliveriesList, Payload: payload,
	})
	if err != nil || result == nil || service.caller != "phone-fulfillment" {
		t.Fatalf("caller identity was not forwarded exactly: caller=%q result=%#v err=%v", service.caller, result, err)
	}
}

func TestCapabilityInvokerRejectsPayloadIdentityAndWrongOwner(t *testing.T) {
	service := &deliveryServiceStub{}
	invoker := NewCapabilityInvoker()
	if err := invoker.Bind(service); err != nil {
		t.Fatal(err)
	}
	for name, invocation := range map[string]capregistry.Invocation{
		"payload customer identity": {
			CallerPluginID: "phone-fulfillment",
			OwnerPluginID:  productdeliverycap.OwnerPluginID, Service: productdeliverycap.ServiceName,
			Version: productdeliverycap.ContractVersion, Method: productdeliverycap.MethodCurrentCustomerDeliveriesList,
			Payload: []byte(`{"limit":20,"customerNo":"CUS_OTHER"}`),
		},
		"wrong owner": {
			CallerPluginID: "phone-fulfillment", OwnerPluginID: "idc-resource", Service: productdeliverycap.ServiceName,
			Version: productdeliverycap.ContractVersion, Method: productdeliverycap.MethodCurrentCustomerDeliveriesList,
			Payload: []byte(`{"limit":20}`),
		},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := invoker.Invoke(context.Background(), invocation); err == nil {
				t.Fatal("unauthorized invocation was accepted")
			}
		})
	}
	if service.caller != "" {
		t.Fatalf("service was reached by a rejected invocation: %q", service.caller)
	}
}
