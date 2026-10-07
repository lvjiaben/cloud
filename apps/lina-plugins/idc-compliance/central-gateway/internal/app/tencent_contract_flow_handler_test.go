// This file verifies guarded Tencent flow, reissue, and termination HTTP handlers.

package app

import (
	"testing"
	"time"

	"lina-platform/idc-compliance-gateway/pkg/processauth"
)

func TestBuildTencentContractFlowUsesOfficialTemplateContract(t *testing.T) {
	now := time.Date(2026, time.August, 16, 10, 0, 0, 0, time.UTC)
	input := validTencentContractFlowRequest(now)
	flow, err := buildTencentContractFlow(input, now)
	if err != nil {
		t.Fatalf("build Tencent contract flow: %v", err)
	}
	if flow.FlowName == nil || *flow.FlowName != "Contract_CTR_0123456789ABCDEF" ||
		flow.TemplateId == nil || *flow.TemplateId != input.TemplateRef ||
		len(flow.FormFields) != 2 || flow.FormFields[0].ComponentId != nil ||
		flow.FormFields[0].ComponentName == nil || *flow.FormFields[0].ComponentName != "合同编号" ||
		len(flow.FlowApprovers) != 2 || flow.FlowApprovers[0].RecipientId == nil ||
		*flow.FlowApprovers[0].RecipientId != input.Approvers[0].RecipientRef ||
		flow.FlowApprovers[0].NotifyType == nil || *flow.FlowApprovers[0].NotifyType != "NONE" ||
		flow.FlowApprovers[1].NotChannelOrganization == nil || *flow.FlowApprovers[1].NotChannelOrganization {
		t.Fatalf("unexpected official Tencent flow: %+v", flow)
	}
}

func TestBuildTencentContractFlowRejectsChangedSignerOrder(t *testing.T) {
	now := time.Date(2026, time.August, 16, 10, 0, 0, 0, time.UTC)
	input := validTencentContractFlowRequest(now)
	input.Approvers[0].SequenceNo = 2
	if flow, err := buildTencentContractFlow(input, now); err == nil || flow != nil {
		t.Fatalf("changed Tencent signer order was accepted: flow=%+v err=%v", flow, err)
	}
}

func TestTencentReleaseTextRejectsMissingReasonAndUnsafeSymbols(t *testing.T) {
	if !validTencentReleaseText("双方协商一致解除", true) ||
		!validTencentReleaseText("费用已结清。", false) {
		t.Fatal("valid Tencent release text was rejected")
	}
	if validTencentReleaseText("", true) || validTencentReleaseText(" 包含首尾空格", true) ||
		validTencentReleaseText("解除😀", true) {
		t.Fatal("unsafe Tencent release text was accepted")
	}
}

func validTencentContractFlowRequest(now time.Time) processauth.TencentContractFlowRequest {
	dynamicTable := `{"headers":[{"content":"服务名称"},{"content":"服务期限"},{"content":"数量"},{"content":"分项金额（CNY）"}],"rowCount":1,"body":{"cells":[{"rowStart":1,"rowEnd":1,"columnStart":1,"columnEnd":1,"content":"独立服务器"},{"rowStart":1,"rowEnd":1,"columnStart":2,"columnEnd":2,"content":"12个月"},{"rowStart":1,"rowEnd":1,"columnStart":3,"columnEnd":3,"content":"1"},{"rowStart":1,"rowEnd":1,"columnStart":4,"columnEnd":4,"content":"1299.00"}]}}`
	return processauth.TencentContractFlowRequest{
		ContractNo: "CTR_0123456789ABCDEF", TemplateRef: "0123456789ABCDEF0123456789ABCDEF",
		LanguageCode: "zh-CN", DeadlineUnix: now.Add(7 * 24 * time.Hour).Unix(),
		FormFields: []processauth.TencentContractFormField{
			{ComponentName: "合同编号", TypeCode: "TEXT", Value: "CTR_0123456789ABCDEF"},
			{ComponentName: "服务明细", TypeCode: "DYNAMIC_TABLE", Value: dynamicTable},
		},
		Approvers: []processauth.TencentContractApprover{
			{SequenceNo: 1, RecipientRef: "customer-recipient", PartyCode: "customer", SubjectTypeCode: "individual", Name: "示例客户", Mobile: "13800001234"},
			{SequenceNo: 2, RecipientRef: "merchant-recipient", PartyCode: "merchant", SubjectTypeCode: "enterprise", OrganizationName: "示例商家有限公司"},
		},
	}
}
