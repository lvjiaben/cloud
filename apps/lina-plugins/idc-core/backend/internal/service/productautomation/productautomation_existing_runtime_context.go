package productautomation

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
)

func (s *serviceImpl) IssueExistingBusinessRuntimeContext(ctx context.Context, input ExistingBusinessRuntimeContextInput) (*ExistingBusinessRuntimeContext, error) {
	if s == nil || s.extensions == nil || s.runtimeContexts == nil || s.consumedFiles == nil || input.InterfaceInstanceNo == "" || input.InterfaceConfigurationRevision < 1 || input.Method == "" ||
		!validRuntimeEvidenceRef(input.IntentNo) || !validRuntimeEvidenceRef(input.OrderNo) || !validRuntimeEvidenceRef(input.OrderItemNo) || !validRuntimeEvidenceRef(input.ConfigurationSnapshotNo) {
		return nil, gerror.New("existing-business supplier runtime context is unavailable")
	}
	exact, err := s.extensions.GetProviderExact(ctx, plugincap.BusinessExtensionProviderExactInput{
		PluginID: input.PluginID, ReleaseVersion: input.ReleaseVersion, Point: input.Point,
		ContractVersion: input.ContractVersion, ReleaseDigest: input.ReleaseDigest, DeclarationDigest: input.DeclarationDigest,
	})
	if err != nil || !validProviderEvidence(exact) || exact.PluginID != input.PluginID || exact.ReleaseVersion != input.ReleaseVersion || exact.Point != input.Point || exact.ContractVersion != input.ContractVersion || exact.ReleaseDigest != input.ReleaseDigest || exact.DeclarationDigest != input.DeclarationDigest {
		return nil, gerror.New("existing-business provider evidence is unavailable")
	}
	view, err := s.loadRuntimeConfigurationView(ctx, exact, dynamicCandidateScope{interfaceInstanceNo: input.InterfaceInstanceNo, configurationRevision: input.InterfaceConfigurationRevision})
	if err != nil {
		return nil, err
	}
	issued, err := s.runtimeContexts.issue(exact, dynamicCandidateScope{interfaceInstanceNo: input.InterfaceInstanceNo, configurationRevision: input.InterfaceConfigurationRevision}, capmodel.BusinessExtensionInvocationPurposeExistingBusiness, input.Method, view, input)
	if err != nil {
		return nil, err
	}
	return &ExistingBusinessRuntimeContext{ContextID: issued.id, Release: func() { s.releaseRuntimeContext(issued.id) }}, nil
}

func validRuntimeEvidenceRef(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for _, character := range value {
		if character >= 'A' && character <= 'Z' || character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || character == '_' || character == '-' {
			continue
		}
		return false
	}
	return true
}
