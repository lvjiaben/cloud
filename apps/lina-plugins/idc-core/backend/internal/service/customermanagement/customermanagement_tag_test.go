package customermanagement

import (
	"context"
	"errors"
	"strings"
	"testing"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/service/customer"
)

type tagMutationTestPersistence struct {
	input  normalizedTagBatch
	result *TagBatchResult
	err    error
	calls  int
}

func (p *tagMutationTestPersistence) applyTagBatch(
	_ context.Context,
	input normalizedTagBatch,
	_ idccap.BusinessAuditWriter,
) (*TagBatchResult, error) {
	p.calls++
	p.input = input
	return p.result, p.err
}

type tagMutationTestAuditWriter struct{}

func (*tagMutationTestAuditWriter) AppendBusinessTransition(
	context.Context,
	idccap.BusinessAuditTransition,
) error {
	return errors.New("unexpected single audit transition")
}

func (*tagMutationTestAuditWriter) AppendBusinessBatchTransition(
	context.Context,
	idccap.BusinessAuditBatchTransition,
) (idccap.BusinessAuditAppendResult, error) {
	return idccap.BusinessAuditAppendResult{}, errors.New("unexpected direct batch audit transition")
}

func TestBatchTagMutationAuthorizesBeforeValidationAndPersistence(t *testing.T) {
	persistence := &tagMutationTestPersistence{}
	service, err := newMutationService(
		managementTestBizCtx{current: bizctxcap.CurrentContext{
			UserID: 71, TenantID: 0, DataScope: 2,
		}},
		&managementTestPrincipal{audience: activeStaffAudience()},
		&managementTestAuthz{grants: map[authz.PermissionKey]bool{}},
		&tagMutationTestAuditWriter{},
		persistence,
	)
	if err != nil {
		t.Fatalf("create denied tag mutation service: %v", err)
	}
	if _, err = service.BatchUpdateTags(context.Background(), TagBatchInput{}); !bizerr.Is(err, customer.CodeNotAccessible) || persistence.calls != 0 {
		t.Fatalf("denied batch mutation calls=%d err=%v", persistence.calls, err)
	}
}

func TestBatchTagMutationCanonicalizesBoundedInput(t *testing.T) {
	persistence := &tagMutationTestPersistence{result: &TagBatchResult{
		AuditNo: "AUD_11111111111111111111111111111111", CustomerCount: 2, TagCount: 2,
	}}
	service, err := newMutationService(
		managementTestBizCtx{current: bizctxcap.CurrentContext{
			UserID: 71, TenantID: 0, DataScope: 2,
		}},
		&managementTestPrincipal{audience: activeStaffAudience()},
		&managementTestAuthz{grants: map[authz.PermissionKey]bool{
			authz.PermissionKey(permissionCustomerTagBatch): true,
		}},
		&tagMutationTestAuditWriter{},
		persistence,
	)
	if err != nil {
		t.Fatalf("create tag mutation service: %v", err)
	}
	result, err := service.BatchUpdateTags(context.Background(), TagBatchInput{
		Action: TagBatchAssign,
		CustomerNos: []string{
			"CUS_22222222222222222222222222222222",
			"CUS_11111111111111111111111111111111",
		},
		TagNos: []string{
			"TAG_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			"TAG_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
		Reason: "  运营   调整  ", IdempotencyKey: "batch-test-0001",
	})
	if err != nil || result != persistence.result || persistence.calls != 1 {
		t.Fatalf("batch tag result=%+v persistence=%+v err=%v", result, persistence, err)
	}
	if persistence.input.customerNos[0] != "CUS_11111111111111111111111111111111" ||
		persistence.input.tagNos[0] != "TAG_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" ||
		persistence.input.reason != "运营 调整" || persistence.input.actorUserID != 71 ||
		!strings.HasPrefix(persistence.input.targetNo, "TGB_") {
		t.Fatalf("unexpected normalized batch tag input: %+v", persistence.input)
	}
}

func TestNormalizeTagBatchRejectsDuplicatesAndUnboundedValues(t *testing.T) {
	validCustomer := "CUS_11111111111111111111111111111111"
	validTag := "TAG_11111111111111111111111111111111"
	valid := TagBatchInput{
		Action: TagBatchAssign, CustomerNos: []string{validCustomer}, TagNos: []string{validTag},
		Reason: "manual update", IdempotencyKey: "batch-test-0001",
	}
	inputs := []TagBatchInput{
		{},
		{Action: TagBatchAssign, CustomerNos: []string{validCustomer, validCustomer}, TagNos: valid.TagNos, Reason: valid.Reason, IdempotencyKey: valid.IdempotencyKey},
		{Action: TagBatchAssign, CustomerNos: valid.CustomerNos, TagNos: []string{validTag, validTag}, Reason: valid.Reason, IdempotencyKey: valid.IdempotencyKey},
		{Action: TagBatchAssign, CustomerNos: valid.CustomerNos, TagNos: valid.TagNos, Reason: strings.Repeat("界", maximumTagBatchReasonRunes+1), IdempotencyKey: valid.IdempotencyKey},
		{Action: TagBatchAssign, CustomerNos: valid.CustomerNos, TagNos: valid.TagNos, Reason: valid.Reason, IdempotencyKey: "short"},
	}
	for _, input := range inputs {
		if _, err := normalizeTagBatch(input); !bizerr.Is(err, customer.CodeProfileInvalid) {
			t.Fatalf("invalid batch tag input %+v returned %v", input, err)
		}
	}
}
