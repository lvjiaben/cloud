// This file verifies the safe manual electronic-contract API projection.

package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/electroniccontractpreparation"
	"lina-plugin-idc-core/backend/cap/contractcap"
)

func TestElectronicContractCreateMapsImmutablePreview(t *testing.T) {
	createdAt := time.Date(2026, time.August, 16, 8, 30, 0, 123_000_000, time.UTC)
	prepared := &electroniccontractpreparation.PreparedContract{
		ContractNo: "CTR_0123456789ABCDEF", OrderNo: "ORDER_20260815-003",
		CustomerType: contractcap.CustomerTypeIndividual,
		Template: contractcap.TemplateSnapshot{
			TemplateNo: "ctpl_0123456789abcdef0123456789abcdef", VersionNo: 5,
			Name: "Optional service contract", LanguageCode: "en-US",
			SigningOrderCode: "customer_first",
		},
		SigningOrderCode: "customer_first", AmountMinor: 129900, CurrencyCode: "CNY",
		CreatedAt: createdAt, SigningExpiresAt: createdAt.Add(7 * 24 * time.Hour),
		RequiredForFulfillment: false,
		Version:                1,
		Preview: &contractcap.PreviewVariables{
			CustomerName: []byte("Example Customer"), MerchantName: []byte("Example IDC Limited"),
			SigningDate: "", SigningDateNoticeCode: "filled_after_authoritative_signature",
			Items: []contractcap.PreviewItem{{
				OrderItemNo: "ITEM_001", ServiceName: "Dedicated Server",
				Quantity: 1, PurchaseCycles: 1, PeriodUnitCode: "month",
				PeriodCount: 12, AmountMinor: 129900,
			}},
		},
	}
	stub := &electronicContractPreparationStub{result: prepared}
	controller, err := NewV1(
		&providerConfigurationServiceStub{}, &enterpriseManagementServiceStub{},
		&supplierConfigurationStub{}, &electronicContractOnboardingStub{}, stub,
		&electronicContractTemplateStub{},
	)
	if err != nil {
		t.Fatal(err)
	}
	request := &v1.ElectronicContractCreateReq{
		OrderNo:        "ORDER_20260815-003",
		TemplateNo:     "ctpl_0123456789abcdef0123456789abcdef",
		IdempotencyKey: "contract-create-20260816-optional-0001",
	}
	result, err := controller.ElectronicContractCreate(context.Background(), request)
	if err != nil || result == nil || result.Item == nil {
		t.Fatalf("create result=%+v err=%v", result, err)
	}
	item := result.Item
	if item.TotalAmountMinor != "129900" || len(item.Lines) != 1 ||
		item.Lines[0].AmountMinor != "129900" || item.Lines[0].Quantity != 1 ||
		item.Lines[0].ServiceName != "Dedicated Server" ||
		item.SigningDate != "" || item.SigningExpiresAt != createdAt.Add(7*24*time.Hour).UnixMilli() ||
		item.RequiredForFulfillment || item.StatusCode != "pending_launch" || item.Version != 1 ||
		stub.input.OrderNo != request.OrderNo || stub.input.TemplateNo != request.TemplateNo ||
		stub.input.IdempotencyKey != request.IdempotencyKey {
		t.Fatalf("unexpected contract projection=%+v input=%+v", item, stub.input)
	}
	wireLine, err := json.Marshal(item.Lines[0])
	if err != nil {
		t.Fatalf("marshal contract line: %v", err)
	}
	if !bytes.Contains(wireLine, []byte(`"quantity":1`)) ||
		bytes.Contains(wireLine, []byte("serverQuantity")) {
		t.Fatalf("contract line still exposes a server-specific quantity: %s", wireLine)
	}
	if prepared.Preview != nil {
		t.Fatal("decrypted contract preview was not cleared after response mapping")
	}
}

func TestElectronicContractTemplateCandidateListMapsSafeMetadata(t *testing.T) {
	stub := &electronicContractPreparationStub{
		templates: []electroniccontractpreparation.ManualOptionalTemplate{{
			TemplateNo: "ctpl_0123456789abcdef0123456789abcdef", VersionNo: 5,
			Name: "Optional service contract", ScopeCode: "all_customers",
			SigningOrderCode: "customer_first", LanguageCode: "en-US",
		}},
	}
	controller, err := NewV1(
		&providerConfigurationServiceStub{}, &enterpriseManagementServiceStub{},
		&supplierConfigurationStub{}, &electronicContractOnboardingStub{}, stub,
		&electronicContractTemplateStub{},
	)
	if err != nil {
		t.Fatal(err)
	}
	request := &v1.ElectronicContractTemplateCandidateListReq{OrderNo: "ORDER_20260815-003"}
	result, err := controller.ElectronicContractTemplateCandidateList(context.Background(), request)
	if err != nil || result == nil || len(result.Items) != 1 {
		t.Fatalf("candidate result=%+v err=%v", result, err)
	}
	item := result.Items[0]
	if stub.templateListOrderNo != request.OrderNo ||
		item.TemplateNo != "ctpl_0123456789abcdef0123456789abcdef" ||
		item.TemplateVersion != 5 || item.LanguageCode != "en-US" ||
		item.ScopeCode != "all_customers" || item.SigningOrderCode != "customer_first" {
		t.Fatalf("unexpected candidate=%+v order=%q", item, stub.templateListOrderNo)
	}
}

func TestElectronicContractSigningSubmissionIsSeparateFromPreviewCreation(t *testing.T) {
	stub := &electronicContractPreparationStub{submitted: &electroniccontractpreparation.SubmittedSigning{
		ContractNo: "CTR_0123456789ABCDEF", FlowNo: "CFW_0123456789ABCDEF0123456789ABCDEF",
		StatusCode: "signing", Version: 2,
	}}
	controller, err := NewV1(
		&providerConfigurationServiceStub{}, &enterpriseManagementServiceStub{},
		&supplierConfigurationStub{}, &electronicContractOnboardingStub{}, stub,
		&electronicContractTemplateStub{},
	)
	if err != nil {
		t.Fatal(err)
	}
	request := &v1.ElectronicContractSigningSubmissionCreateReq{
		ContractNo: "CTR_0123456789ABCDEF", ExpectedVersion: 1,
		IdempotencyKey: "contract-submit-20260816-0001",
	}
	result, err := controller.ElectronicContractSigningSubmissionCreate(context.Background(), request)
	if err != nil || result == nil || result.Item == nil || result.Item.StatusCode != "signing" ||
		result.Item.FlowNo != stub.submitted.FlowNo || stub.submitInput.ContractNo != request.ContractNo ||
		stub.submitInput.ExpectedVersion != 1 || stub.submitInput.IdempotencyKey != request.IdempotencyKey {
		t.Fatalf("submission=%+v input=%+v err=%v", result, stub.submitInput, err)
	}
}

func TestElectronicContractReplacementPreviewDoesNotSubmitTencent(t *testing.T) {
	createdAt := time.Date(2026, time.August, 17, 8, 30, 0, 0, time.UTC)
	prepared := &electroniccontractpreparation.PreparedContract{
		ContractNo: "CTR_1123456789ABCDEF", OrderNo: "ORDER_20260815-003",
		CustomerType: contractcap.CustomerTypeIndividual,
		Template: contractcap.TemplateSnapshot{
			TemplateNo: "ctpl_0123456789abcdef0123456789abcdef", VersionNo: 6,
			Name: "Replacement service contract", LanguageCode: "en-US",
			SigningOrderCode: "customer_first",
		},
		SigningOrderCode: "customer_first", AmountMinor: 129900, CurrencyCode: "CNY",
		CreatedAt: createdAt, SigningExpiresAt: createdAt.Add(7 * 24 * time.Hour), Version: 1,
		Preview: &contractcap.PreviewVariables{
			CustomerName: []byte("Example Customer"), MerchantName: []byte("Example Merchant"),
			SigningDateNoticeCode: "filled_after_authoritative_signature",
		},
	}
	stub := &electronicContractPreparationStub{result: prepared}
	controller, err := NewV1(
		&providerConfigurationServiceStub{}, &enterpriseManagementServiceStub{},
		&supplierConfigurationStub{}, &electronicContractOnboardingStub{}, stub,
		&electronicContractTemplateStub{},
	)
	if err != nil {
		t.Fatal(err)
	}
	request := &v1.ElectronicContractReplacementPreviewCreateReq{
		ContractNo:     "CTR_0123456789ABCDEF",
		TemplateNo:     "ctpl_0123456789abcdef0123456789abcdef",
		IdempotencyKey: "contract-replacement-20260817-0001",
	}
	result, err := controller.ElectronicContractReplacementPreviewCreate(context.Background(), request)
	if err != nil || result == nil || result.Item == nil ||
		result.Item.ContractNo != prepared.ContractNo || stub.replacementInput.ContractNo != request.ContractNo ||
		stub.replacementInput.TemplateNo != request.TemplateNo || stub.submitInput.ContractNo != "" {
		t.Fatalf("replacement preview=%+v input=%+v submit=%+v err=%v",
			result, stub.replacementInput, stub.submitInput, err)
	}
}

func TestElectronicContractReplacementOptionListMapsOnlySafeChoices(t *testing.T) {
	stub := &electronicContractPreparationStub{
		replacementOptions: &electroniccontractpreparation.ReplacementOptions{
			ContractNo: "CTR_0123456789ABCDEF", OrderNo: "ORDER_20260815-003",
			CustomerNo: "CUS_0123456789ABCDEF", CustomerType: contractcap.CustomerTypeEnterprise,
			Templates: []electroniccontractpreparation.ManualOptionalTemplate{{
				TemplateNo: "ctpl_0123456789abcdef0123456789abcdef", VersionNo: 6,
				Name: "Replacement service contract", ScopeCode: "enterprise",
				SigningOrderCode: "customer_first", LanguageCode: "zh-CN",
			}},
			Signers: []electroniccontractpreparation.ReplacementSigner{{
				SignerNo:   "csgn_0123456789abcdef0123456789abcdef",
				MaskedName: "张*", MaskedPhone: "138****8000",
			}},
		},
	}
	controller, err := NewV1(
		&providerConfigurationServiceStub{}, &enterpriseManagementServiceStub{},
		&supplierConfigurationStub{}, &electronicContractOnboardingStub{}, stub,
		&electronicContractTemplateStub{},
	)
	if err != nil {
		t.Fatal(err)
	}
	request := &v1.ElectronicContractReplacementOptionListReq{ContractNo: "CTR_0123456789ABCDEF"}
	result, err := controller.ElectronicContractReplacementOptionList(context.Background(), request)
	if err != nil || result == nil || len(result.Templates) != 1 || len(result.Signers) != 1 {
		t.Fatalf("replacement options=%+v err=%v", result, err)
	}
	if stub.replacementOptionNo != request.ContractNo ||
		result.CustomerType != "enterprise" ||
		result.Templates[0].TemplateVersion != 6 ||
		result.Signers[0].MaskedPhone != "138****8000" {
		t.Fatalf("unexpected replacement options=%+v requested=%q", result, stub.replacementOptionNo)
	}
}

func TestElectronicContractReissueAndTerminationMapIndependentCommands(t *testing.T) {
	stub := &electronicContractPreparationStub{submitted: &electroniccontractpreparation.SubmittedSigning{
		ContractNo: "CTR_0123456789ABCDEF", FlowNo: "CFW_1123456789ABCDEF0123456789ABCDEF",
		StatusCode: "signing", Version: 5,
	}}
	controller, err := NewV1(
		&providerConfigurationServiceStub{}, &enterpriseManagementServiceStub{},
		&supplierConfigurationStub{}, &electronicContractOnboardingStub{}, stub,
		&electronicContractTemplateStub{},
	)
	if err != nil {
		t.Fatal(err)
	}
	reissueRequest := &v1.ElectronicContractReissueCreateReq{
		ContractNo: "CTR_0123456789ABCDEF", ExpectedVersion: 3,
		IdempotencyKey: "contract-reissue-20260817-0001",
	}
	reissue, err := controller.ElectronicContractReissueCreate(context.Background(), reissueRequest)
	if err != nil || reissue == nil || reissue.Item == nil ||
		stub.reissueInput.ContractNo != reissueRequest.ContractNo ||
		stub.reissueInput.ExpectedVersion != reissueRequest.ExpectedVersion {
		t.Fatalf("reissue=%+v input=%+v err=%v", reissue, stub.reissueInput, err)
	}
	terminationRequest := &v1.ElectronicContractTerminationCreateReq{
		ContractNo: "CTR_0123456789ABCDEF", ExpectedVersion: 4,
		Reason: "双方协商一致解除", RemainInForceItem: "保密条款继续有效",
		OriginalExpenseSettlement: "费用已结清", Confirmed: true,
		IdempotencyKey: "contract-termination-20260817-0001",
	}
	termination, err := controller.ElectronicContractTerminationCreate(context.Background(), terminationRequest)
	if err != nil || termination == nil || termination.Item == nil ||
		stub.terminationInput.ContractNo != terminationRequest.ContractNo ||
		stub.terminationInput.Reason != terminationRequest.Reason ||
		stub.terminationInput.RemainInForceItem != terminationRequest.RemainInForceItem ||
		!stub.terminationInput.Confirmed {
		t.Fatalf("termination=%+v input=%+v err=%v", termination, stub.terminationInput, err)
	}
}
