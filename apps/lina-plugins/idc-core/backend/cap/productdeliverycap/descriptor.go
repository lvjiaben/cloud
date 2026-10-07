package productdeliverycap

import (
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/capregistry"
)

const (
	CapabilityCurrentCustomerDeliveriesList = "host:idc:product-delivery.current-customer-deliveries.list"
	CapabilityCurrentCustomerDeliveryGet    = "host:idc:product-delivery.current-customer-delivery.get"
)

func NewDescriptor(invoker capregistry.Invoker) (capregistry.Descriptor, error) {
	if invoker == nil {
		return capregistry.Descriptor{}, gerror.New("productdeliverycap: descriptor invoker is required")
	}
	return capregistry.Descriptor{
		OwnerPluginID:   OwnerPluginID,
		Service:         ServiceName,
		Version:         ContractVersion,
		SourceContract:  SourceContractPackage,
		DynamicContract: DynamicContractPackage,
		Invoker:         invoker,
		Methods: []capregistry.MethodDescriptor{
			{Method: MethodCurrentCustomerDeliveriesList, Capability: CapabilityCurrentCustomerDeliveriesList, Risk: capregistry.RiskLevelRead, ResourceKind: capregistry.ResourceKindNone, RequestPayload: "productdeliverycap.CurrentCustomerDeliveryListRequest", ResponsePayload: "productdeliverycap.CurrentCustomerDeliveryPage"},
			{Method: MethodCurrentCustomerDeliveryGet, Capability: CapabilityCurrentCustomerDeliveryGet, Risk: capregistry.RiskLevelRead, ResourceKind: capregistry.ResourceKindNone, RequestPayload: "productdeliverycap.CurrentCustomerDeliveryGetRequest", ResponsePayload: "productdeliverycap.CurrentCustomerDeliveryResult"},
		},
	}, nil
}
