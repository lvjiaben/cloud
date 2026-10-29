package inboxcap

import (
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/capregistry"
)

func NewDescriptor(invoker capregistry.Invoker) (capregistry.Descriptor, error) {
	if invoker == nil {
		return capregistry.Descriptor{}, gerror.New("inboxcap: descriptor invoker is required")
	}
	return capregistry.Descriptor{
		OwnerPluginID:   OwnerPluginID,
		Service:         ServiceName,
		Version:         ContractVersion,
		SourceContract:  SourceContractPackage,
		DynamicContract: DynamicContractPackage,
		Invoker:         invoker,
		Methods: []capregistry.MethodDescriptor{{
			Method: MethodPublish, Capability: "host:shared:inbox.publish",
			Risk: capregistry.RiskLevelWrite, ResourceKind: capregistry.ResourceKindNone,
			RequestPayload: "inboxcap.PublishRequest", ResponsePayload: "inboxcap.PublishResult",
		}, {Method: MethodCurrentSync, Capability: "host:shared:inbox.currentSync", Risk: capregistry.RiskLevelWrite, ResourceKind: capregistry.ResourceKindNone, RequestPayload: "inboxcap.CurrentSyncRequest", ResponsePayload: "inboxcap.CurrentSyncResult"}},
	}, nil
}
