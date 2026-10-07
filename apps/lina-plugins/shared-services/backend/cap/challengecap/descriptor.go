package challengecap

import (
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/capregistry"
)

func NewDescriptor(invoker capregistry.Invoker) (capregistry.Descriptor, error) {
	if invoker == nil {
		return capregistry.Descriptor{}, gerror.New("challengecap: descriptor invoker is required")
	}
	return capregistry.Descriptor{
		OwnerPluginID:   OwnerPluginID,
		Service:         ServiceName,
		Version:         ContractVersion,
		SourceContract:  SourceContractPackage,
		DynamicContract: DynamicContractPackage,
		Invoker:         invoker,
		Methods: []capregistry.MethodDescriptor{
			{Method: string(MethodCreate), Capability: "host:shared:challenge.create", Risk: capregistry.RiskLevelWrite, ResourceKind: capregistry.ResourceKindNone, RequestPayload: "challengecap.CreateRequest", ResponsePayload: "challengecap.Presentation"},
			{Method: string(MethodComplete), Capability: "host:shared:challenge.complete", Risk: capregistry.RiskLevelWrite, ResourceKind: capregistry.ResourceKindNone, RequestPayload: "challengecap.CompleteRequest", ResponsePayload: "challengecap.Proof"},
			{Method: string(MethodConsume), Capability: "host:shared:challenge.consume", Risk: capregistry.RiskLevelWrite, ResourceKind: capregistry.ResourceKindNone, RequestPayload: "challengecap.ConsumeRequest", ResponsePayload: "challengecap.ConsumeResult"},
		},
	}, nil
}
