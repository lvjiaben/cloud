// This file tests safe capability projection for configured, unavailable, and
// operation-dependent Tencent abilities.

package tencentess

import (
	"context"
	"testing"
	"time"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	essbasic "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/essbasic/v20210526"
)

func TestCheckCapabilitiesSeparatesReadOnlyProofFromPerOperationChecks(t *testing.T) {
	api := inertAPI()
	api.templates = func(context.Context, *essbasic.DescribeTemplatesRequest) (*essbasic.DescribeTemplatesResponse, error) {
		response := essbasic.NewDescribeTemplatesResponse()
		response.Response = &essbasic.DescribeTemplatesResponseParams{RequestId: common.StringPtr("templates")}
		return response, nil
	}
	api.autoSign = func(context.Context, *essbasic.DescribeExtendedServiceAuthInfoRequest) (*essbasic.DescribeExtendedServiceAuthInfoResponse, error) {
		response := essbasic.NewDescribeExtendedServiceAuthInfoResponse()
		response.Response = &essbasic.DescribeExtendedServiceAuthInfoResponseParams{
			AuthInfo: []*essbasic.ExtentServiceAuthInfo{
				{Type: common.StringPtr(extendedServiceAutoSign), Status: common.StringPtr(extendedServiceEnabled)},
				{Type: common.StringPtr(extendedServiceDownload), Status: common.StringPtr(extendedServiceEnabled)},
			},
			RequestId: common.StringPtr("extended-services"),
		}
		return response, nil
	}
	api.usage = func(context.Context, *essbasic.ChannelDescribeBillUsageDetailRequest) (*essbasic.ChannelDescribeBillUsageDetailResponse, error) {
		response := essbasic.NewChannelDescribeBillUsageDetailResponse()
		response.Response = &essbasic.ChannelDescribeBillUsageDetailResponseParams{
			Total: common.Int64Ptr(0), RequestId: common.StringPtr("usage"),
		}
		return response, nil
	}
	api.accountBill = func(context.Context, *essbasic.ChannelDescribeAccountBillDetailRequest) (*essbasic.ChannelDescribeAccountBillDetailResponse, error) {
		response := essbasic.NewChannelDescribeAccountBillDetailResponse()
		response.Response = &essbasic.ChannelDescribeAccountBillDetailResponseParams{
			BoundAccountsNumber: common.Int64Ptr(0), RemainAvailableAccountsNumber: common.Int64Ptr(0),
			RequestId: common.StringPtr("account-bill"),
		}
		return response, nil
	}
	guard := &fakeWriteGuard{}
	client := testClient(t, api, guard,
		CapabilityTemplateRead, CapabilityFlowCreate, CapabilityFlowRelease, CapabilitySignURL, CapabilityFlowQuery,
		CapabilityDownload, CapabilityEvidenceReport, CapabilityAutoSign, CapabilityUsageRead, CapabilityAccountBill,
	)
	checkedAt := time.Date(2026, 8, 14, 2, 3, 4, 0, time.UTC)
	client.now = func() time.Time { return checkedAt }
	statuses := client.CheckCapabilities(context.Background())
	if len(statuses) != 10 {
		t.Fatalf("status count=%d want=10", len(statuses))
	}
	available := 0
	operationChecks := 0
	for _, status := range statuses {
		if status.CheckedAt != checkedAt || status.Code != "" {
			t.Fatalf("unexpected safe status: %+v", status)
		}
		switch status.State {
		case CapabilityAvailable:
			available++
		case CapabilityOperationCheckRequired:
			operationChecks++
		default:
			t.Fatalf("unexpected capability state: %+v", status)
		}
	}
	if available != 5 || operationChecks != 5 {
		t.Fatalf("available=%d operationChecks=%d", available, operationChecks)
	}
}

func TestCheckCapabilitiesFailsClosedForDisabledExtendedService(t *testing.T) {
	api := inertAPI()
	api.autoSign = func(context.Context, *essbasic.DescribeExtendedServiceAuthInfoRequest) (*essbasic.DescribeExtendedServiceAuthInfoResponse, error) {
		response := essbasic.NewDescribeExtendedServiceAuthInfoResponse()
		response.Response = &essbasic.DescribeExtendedServiceAuthInfoResponseParams{
			AuthInfo: []*essbasic.ExtentServiceAuthInfo{{
				Type: common.StringPtr(extendedServiceAutoSign), Status: common.StringPtr(extendedServiceDisabled),
			}},
			RequestId: common.StringPtr("extended-services"),
		}
		return response, nil
	}
	client := testClient(t, api, nil, CapabilityAutoSign)
	statuses := client.CheckCapabilities(context.Background())
	for _, status := range statuses {
		if status.Capability == CapabilityAutoSign {
			if status.State != CapabilityUnavailable || status.Code != ErrorPermissionDenied {
				t.Fatalf("unexpected disabled auto-sign status: %+v", status)
			}
			return
		}
	}
	t.Fatal("auto-sign capability status was not returned")
}
