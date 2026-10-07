package supplierconfig

import "testing"

func TestSubjectAdapterGateKeepsTencentAndNuoNuoIndependent(t *testing.T) {
	tests := []struct {
		name         string
		provider     string
		operation    OperationKind
		result       OperationResult
		wantEvidence string
		wantEnabled  bool
	}{
		{
			name: "Tencent active binding", provider: ProviderTencentESSBasic,
			operation: OperationSubjectBind, result: OperationResultSucceeded,
			wantEvidence: EvidenceOfficialPublicContract, wantEnabled: true,
		},
		{
			name: "Tencent pending binding", provider: ProviderTencentESSBasic,
			operation: OperationSubjectBind, result: OperationResultPending,
			wantEvidence: EvidenceOfficialPublicContract, wantEnabled: false,
		},
		{
			name: "Tencent revoked binding", provider: ProviderTencentESSBasic,
			operation: OperationSubjectRevoke, result: OperationResultSucceeded,
			wantEvidence: EvidenceOfficialPublicContract, wantEnabled: false,
		},
		{
			name: "NuoNuo remains externally gated", provider: ProviderNuonuoInvoice,
			operation: OperationSubjectBind, result: OperationResultSucceeded,
			wantEvidence: EvidenceExternalConfirmationRequired, wantEnabled: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			evidence, enabled := subjectAdapterGate(test.provider, test.operation, test.result)
			if evidence != test.wantEvidence || enabled != test.wantEnabled {
				t.Fatalf("gate=(%q,%v) want=(%q,%v)", evidence, enabled, test.wantEvidence, test.wantEnabled)
			}
		})
	}
}
