// This file tests fixed SDK settings, guarded writes, bounded reads, recovery,
// result ambiguity, and reconciliation projections.

package tencentess

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	tencenterrors "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	essbasic "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/essbasic/v20210526"

	"lina-platform/idc-compliance-gateway/internal/suppliercallback"
	"lina-platform/idc-compliance-gateway/internal/supplierguard"
)

func validAgent(agent *essbasic.Agent) bool {
	return agent != nil && validOpaque(stringValue(agent.AppId), 1, 64) &&
		validOpaque(stringValue(agent.ProxyOrganizationOpenId), 1, 64) &&
		agent.ProxyOperator != nil && validOpaque(stringValue(agent.ProxyOperator.OpenId), 1, 64)
}

const testFlowID = "AbCdEfGhIjKlMnOpQrStUvWxYz012345"

type fakeOfficialAPI struct {
	templates      func(context.Context, *essbasic.DescribeTemplatesRequest) (*essbasic.DescribeTemplatesResponse, error)
	create         func(context.Context, *essbasic.CreateFlowsByTemplatesRequest) (*essbasic.CreateFlowsByTemplatesResponse, error)
	release        func(context.Context, *essbasic.ChannelCreateReleaseFlowRequest) (*essbasic.ChannelCreateReleaseFlowResponse, error)
	signURL        func(context.Context, *essbasic.ChannelCreateFlowSignUrlRequest) (*essbasic.ChannelCreateFlowSignUrlResponse, error)
	flows          func(context.Context, *essbasic.DescribeFlowDetailInfoRequest) (*essbasic.DescribeFlowDetailInfoResponse, error)
	download       func(context.Context, *essbasic.GetDownloadFlowUrlRequest) (*essbasic.GetDownloadFlowUrlResponse, error)
	resources      func(context.Context, *essbasic.DescribeResourceUrlsByFlowsRequest) (*essbasic.DescribeResourceUrlsByFlowsResponse, error)
	createReport   func(context.Context, *essbasic.CreateChannelFlowEvidenceReportRequest) (*essbasic.CreateChannelFlowEvidenceReportResponse, error)
	describeReport func(context.Context, *essbasic.DescribeChannelFlowEvidenceReportRequest) (*essbasic.DescribeChannelFlowEvidenceReportResponse, error)
	autoSign       func(context.Context, *essbasic.DescribeExtendedServiceAuthInfoRequest) (*essbasic.DescribeExtendedServiceAuthInfoResponse, error)
	usage          func(context.Context, *essbasic.ChannelDescribeBillUsageDetailRequest) (*essbasic.ChannelDescribeBillUsageDetailResponse, error)
	accountBill    func(context.Context, *essbasic.ChannelDescribeAccountBillDetailRequest) (*essbasic.ChannelDescribeAccountBillDetailResponse, error)
}

func (f *fakeOfficialAPI) DescribeTemplatesWithContext(ctx context.Context, request *essbasic.DescribeTemplatesRequest) (*essbasic.DescribeTemplatesResponse, error) {
	return f.templates(ctx, request)
}

func (f *fakeOfficialAPI) CreateFlowsByTemplatesWithContext(ctx context.Context, request *essbasic.CreateFlowsByTemplatesRequest) (*essbasic.CreateFlowsByTemplatesResponse, error) {
	return f.create(ctx, request)
}

func (f *fakeOfficialAPI) ChannelCreateReleaseFlowWithContext(ctx context.Context, request *essbasic.ChannelCreateReleaseFlowRequest) (*essbasic.ChannelCreateReleaseFlowResponse, error) {
	return f.release(ctx, request)
}

func (f *fakeOfficialAPI) ChannelCreateFlowSignUrlWithContext(ctx context.Context, request *essbasic.ChannelCreateFlowSignUrlRequest) (*essbasic.ChannelCreateFlowSignUrlResponse, error) {
	return f.signURL(ctx, request)
}

func (f *fakeOfficialAPI) DescribeFlowDetailInfoWithContext(ctx context.Context, request *essbasic.DescribeFlowDetailInfoRequest) (*essbasic.DescribeFlowDetailInfoResponse, error) {
	return f.flows(ctx, request)
}

func (f *fakeOfficialAPI) GetDownloadFlowUrlWithContext(ctx context.Context, request *essbasic.GetDownloadFlowUrlRequest) (*essbasic.GetDownloadFlowUrlResponse, error) {
	return f.download(ctx, request)
}

func (f *fakeOfficialAPI) DescribeResourceUrlsByFlowsWithContext(ctx context.Context, request *essbasic.DescribeResourceUrlsByFlowsRequest) (*essbasic.DescribeResourceUrlsByFlowsResponse, error) {
	return f.resources(ctx, request)
}

func (f *fakeOfficialAPI) CreateChannelFlowEvidenceReportWithContext(ctx context.Context, request *essbasic.CreateChannelFlowEvidenceReportRequest) (*essbasic.CreateChannelFlowEvidenceReportResponse, error) {
	return f.createReport(ctx, request)
}

func (f *fakeOfficialAPI) DescribeChannelFlowEvidenceReportWithContext(ctx context.Context, request *essbasic.DescribeChannelFlowEvidenceReportRequest) (*essbasic.DescribeChannelFlowEvidenceReportResponse, error) {
	return f.describeReport(ctx, request)
}

func (f *fakeOfficialAPI) DescribeExtendedServiceAuthInfoWithContext(ctx context.Context, request *essbasic.DescribeExtendedServiceAuthInfoRequest) (*essbasic.DescribeExtendedServiceAuthInfoResponse, error) {
	return f.autoSign(ctx, request)
}

func (f *fakeOfficialAPI) ChannelDescribeBillUsageDetailWithContext(ctx context.Context, request *essbasic.ChannelDescribeBillUsageDetailRequest) (*essbasic.ChannelDescribeBillUsageDetailResponse, error) {
	return f.usage(ctx, request)
}

func (f *fakeOfficialAPI) ChannelDescribeAccountBillDetailWithContext(ctx context.Context, request *essbasic.ChannelDescribeAccountBillDetailRequest) (*essbasic.ChannelDescribeAccountBillDetailResponse, error) {
	return f.accountBill(ctx, request)
}

type fakeWriteGuard struct {
	lease       supplierguard.Lease
	acquired    supplierguard.Admission
	observation supplierguard.Observation
	acquireErr  error
	observeErr  error
}

func (f *fakeWriteGuard) Acquire(_ context.Context, input supplierguard.Admission) (supplierguard.Lease, error) {
	f.acquired = input
	return f.lease, f.acquireErr
}

func (f *fakeWriteGuard) Observe(_ context.Context, input supplierguard.Observation) (supplierguard.Lease, error) {
	f.observation = input
	return f.lease, f.observeErr
}

type fakeReferenceBinder struct {
	input suppliercallback.ReferenceBinding
	err   error
}

type fakeChargeAdmission struct {
	err   error
	calls int
}

func (f *fakeChargeAdmission) AdmitNewCharge(context.Context, string) error {
	f.calls++
	return f.err
}

type fakeSubjectSource struct {
	payload []byte
	err     error
}

func (f *fakeSubjectSource) WithActiveSubject(
	ctx context.Context,
	provider string,
	callback func([]byte) error,
) error {
	if f.err != nil {
		return f.err
	}
	if provider != providerCode || ctx == nil || callback == nil {
		return errSubjectUnavailable
	}
	return callback(f.payload)
}

func (f *fakeReferenceBinder) BindExternalReference(_ context.Context, input suppliercallback.ReferenceBinding) (int64, error) {
	f.input = input
	return 1, f.err
}

func TestCreateFlowUsesGuardOfficialIdentityAndReferenceBinding(t *testing.T) {
	requestID := "request-id"
	called := 0
	api := inertAPI()
	api.create = func(_ context.Context, request *essbasic.CreateFlowsByTemplatesRequest) (*essbasic.CreateFlowsByTemplatesResponse, error) {
		called++
		if !validAgent(request.Agent) || stringValue(request.Agent.AppId) != "app-id" ||
			len(request.FlowInfos) != 1 || request.NeedPreview == nil || *request.NeedPreview {
			t.Fatalf("unexpected create request: %+v", request)
		}
		response := essbasic.NewCreateFlowsByTemplatesResponse()
		response.Response = &essbasic.CreateFlowsByTemplatesResponseParams{
			FlowIds: []*string{common.StringPtr(testFlowID)}, RequestId: &requestID,
		}
		return response, nil
	}
	guard := &fakeWriteGuard{lease: supplierguard.Lease{
		AttemptNo: "lwa_0123456789abcdef0123456789abcdef",
		RouteNo:   "lwr_0123456789abcdef0123456789abcdef", CallSupplier: true,
	}}
	binder := &fakeReferenceBinder{}
	client := testClient(t, api, guard, CapabilityFlowCreate)
	if err := client.SetReferenceBinder(binder); err != nil {
		t.Fatalf("SetReferenceBinder returned error: %v", err)
	}
	result, err := client.CreateFlow(
		context.Background(), "lbr_0123456789abcdef0123456789abcdef",
		&essbasic.FlowInfo{FlowName: common.StringPtr("contract")}, false,
	)
	if err != nil {
		t.Fatalf("CreateFlow returned error: %v", err)
	}
	if called != 1 || result.FlowID != testFlowID || result.RequestID != requestID ||
		guard.acquired.Provider != providerCode || guard.acquired.OperationScope != OperationFlowCreate ||
		guard.observation.Result != supplierguard.ResultSucceeded ||
		string(binder.input.ExternalReference) != testFlowID || binder.input.RouteNo != result.RouteNo {
		t.Fatalf("unexpected guarded result: result=%+v guard=%+v binder=%+v", result, guard, binder)
	}
}

func TestCreateFlowStopsBeforeSupplierWhenBuyerPlanIsInsufficient(t *testing.T) {
	called := 0
	api := inertAPI()
	api.create = func(context.Context, *essbasic.CreateFlowsByTemplatesRequest) (*essbasic.CreateFlowsByTemplatesResponse, error) {
		called++
		return nil, errors.New("supplier call must not happen")
	}
	guard := &fakeWriteGuard{}
	client := testClient(t, api, guard, CapabilityFlowCreate)
	if err := client.SetReferenceBinder(&fakeReferenceBinder{}); err != nil {
		t.Fatalf("SetReferenceBinder returned error: %v", err)
	}
	gate := &fakeChargeAdmission{err: errors.New("buyer quota exhausted")}
	if err := client.SetChargeAdmission(gate); err != nil {
		t.Fatalf("SetChargeAdmission returned error: %v", err)
	}
	_, err := client.CreateFlow(
		context.Background(), "lbr_0123456789abcdef0123456789abcdef",
		&essbasic.FlowInfo{FlowName: common.StringPtr("contract")}, false,
	)
	var safe *Error
	if !errors.As(err, &safe) || safe.Code != ErrorPermissionDenied {
		t.Fatalf("CreateFlow error=%v", err)
	}
	if gate.calls != 1 || called != 0 || guard.acquired.RequestNo != "" {
		t.Fatalf("gateCalls=%d supplierCalls=%d guard=%+v", gate.calls, called, guard)
	}
}

func TestCreateReleaseFlowUsesOfficialContractAndIndependentGuardScope(t *testing.T) {
	requestID := "release-request-id"
	called := 0
	api := inertAPI()
	api.release = func(_ context.Context, request *essbasic.ChannelCreateReleaseFlowRequest) (*essbasic.ChannelCreateReleaseFlowResponse, error) {
		called++
		if !validAgent(request.Agent) || stringValue(request.NeedRelievedFlowId) != testFlowID ||
			request.ReliveInfo == nil || stringValue(request.ReliveInfo.Reason) != "双方协商一致解除" ||
			request.Deadline == nil {
			t.Fatalf("unexpected release request: %+v", request)
		}
		response := essbasic.NewChannelCreateReleaseFlowResponse()
		response.Response = &essbasic.ChannelCreateReleaseFlowResponseParams{
			FlowId:    common.StringPtr("1234567890AbCdEfGhIjKlMnOpQrStUv"),
			RequestId: &requestID,
		}
		return response, nil
	}
	guard := &fakeWriteGuard{lease: supplierguard.Lease{
		AttemptNo: "lwa_1123456789abcdef0123456789abcdef",
		RouteNo:   "lwr_1123456789abcdef0123456789abcdef", CallSupplier: true,
	}}
	binder := &fakeReferenceBinder{}
	client := testClient(t, api, guard, CapabilityFlowRelease)
	if err := client.SetReferenceBinder(binder); err != nil {
		t.Fatalf("SetReferenceBinder returned error: %v", err)
	}
	deadline := time.Now().UTC().Add(7 * 24 * time.Hour).Unix()
	reason := "双方协商一致解除"
	result, err := client.CreateReleaseFlow(
		context.Background(), "contract-termination-test-0001", testFlowID,
		&essbasic.RelieveInfo{Reason: &reason}, deadline,
	)
	if err != nil {
		t.Fatalf("CreateReleaseFlow returned error: %v", err)
	}
	if called != 1 || result.RequestID != requestID || result.FlowID == "" ||
		guard.acquired.OperationScope != OperationFlowRelease ||
		guard.observation.Result != supplierguard.ResultSucceeded ||
		binder.input.OperationScope != OperationFlowRelease ||
		string(binder.input.ExternalReference) != result.FlowID {
		t.Fatalf("unexpected guarded release result=%+v guard=%+v binder=%+v", result, guard, binder)
	}
}

func TestCreateFlowRecoveryDoesNotRepeatOfficialWrite(t *testing.T) {
	api := inertAPI()
	api.create = func(context.Context, *essbasic.CreateFlowsByTemplatesRequest) (*essbasic.CreateFlowsByTemplatesResponse, error) {
		t.Fatal("recovered request repeated the Tencent write")
		return nil, nil
	}
	guard := &fakeWriteGuard{lease: supplierguard.Lease{
		RouteNo: "lwr_0123456789abcdef0123456789abcdef", Recovered: true,
		Status: "observed", Result: supplierguard.ResultSucceeded,
	}}
	client := testClient(t, api, guard, CapabilityFlowCreate)
	if err := client.SetReferenceBinder(&fakeReferenceBinder{}); err != nil {
		t.Fatalf("SetReferenceBinder returned error: %v", err)
	}
	result, err := client.CreateFlow(
		context.Background(), "lbr_0123456789abcdef0123456789abcdef",
		&essbasic.FlowInfo{FlowName: common.StringPtr("contract")}, false,
	)
	if err != nil || !result.Recovered || result.RouteNo == "" {
		t.Fatalf("unexpected recovery: result=%+v err=%v", result, err)
	}
}

func TestCreateFlowRecoveryPreservesResultUnknown(t *testing.T) {
	api := inertAPI()
	api.create = func(context.Context, *essbasic.CreateFlowsByTemplatesRequest) (*essbasic.CreateFlowsByTemplatesResponse, error) {
		t.Fatal("result-unknown recovery repeated the Tencent write")
		return nil, nil
	}
	guard := &fakeWriteGuard{lease: supplierguard.Lease{
		RouteNo: "lwr_0123456789abcdef0123456789abcdef", Recovered: true,
		Status: "observed", Result: supplierguard.ResultUnknown,
	}}
	client := testClient(t, api, guard, CapabilityFlowCreate)
	if err := client.SetReferenceBinder(&fakeReferenceBinder{}); err != nil {
		t.Fatalf("SetReferenceBinder returned error: %v", err)
	}
	_, err := client.CreateFlow(
		context.Background(), "lbr_0123456789abcdef0123456789abcdef",
		&essbasic.FlowInfo{FlowName: common.StringPtr("contract")}, false,
	)
	var safe *Error
	if !errors.As(err, &safe) || safe.Code != ErrorResultUnknown {
		t.Fatalf("unexpected recovered unknown error: %v", err)
	}
}

func TestCreateFlowTransportFailureIsResultUnknown(t *testing.T) {
	api := inertAPI()
	api.create = func(context.Context, *essbasic.CreateFlowsByTemplatesRequest) (*essbasic.CreateFlowsByTemplatesResponse, error) {
		return nil, context.DeadlineExceeded
	}
	guard := &fakeWriteGuard{lease: supplierguard.Lease{
		AttemptNo: "lwa_0123456789abcdef0123456789abcdef",
		RouteNo:   "lwr_0123456789abcdef0123456789abcdef", CallSupplier: true,
	}}
	client := testClient(t, api, guard, CapabilityFlowCreate)
	if err := client.SetReferenceBinder(&fakeReferenceBinder{}); err != nil {
		t.Fatalf("SetReferenceBinder returned error: %v", err)
	}
	_, err := client.CreateFlow(
		context.Background(), "lbr_0123456789abcdef0123456789abcdef",
		&essbasic.FlowInfo{FlowName: common.StringPtr("contract")}, false,
	)
	var safe *Error
	if !errors.As(err, &safe) || safe.Code != ErrorResultUnknown ||
		guard.observation.Result != supplierguard.ResultUnknown {
		t.Fatalf("unexpected unknown result: err=%v observation=%+v", err, guard.observation)
	}
}

func TestBindLocalMerchantApproverUsesFrozenAgentAndReleasesIt(t *testing.T) {
	organization := "示例商家有限公司"
	approverType := "ORGANIZATION"
	notChannel := false
	flow := &essbasic.FlowInfo{FlowApprovers: []*essbasic.FlowApproverInfo{{
		ApproverType: &approverType, OrganizationName: &organization,
		NotChannelOrganization: &notChannel,
	}}}
	agent := &essbasic.Agent{
		ProxyOrganizationOpenId: common.StringPtr("organization-id"),
		ProxyOperator:           &essbasic.UserInfo{OpenId: common.StringPtr("operator-id")},
	}
	release, err := bindLocalMerchantApprovers(flow, agent)
	if err != nil || flow.FlowApprovers[0].OpenId == nil || *flow.FlowApprovers[0].OpenId != "operator-id" ||
		flow.FlowApprovers[0].OrganizationOpenId == nil || *flow.FlowApprovers[0].OrganizationOpenId != "organization-id" {
		t.Fatalf("merchant approver was not bound: flow=%+v err=%v", flow, err)
	}
	release()
	if flow.FlowApprovers[0].OpenId != nil || flow.FlowApprovers[0].OrganizationOpenId != nil {
		t.Fatal("merchant subject identifiers were not released after the provider call")
	}
}

func TestQueryNormalizesOfficialFlow(t *testing.T) {
	api := inertAPI()
	api.flows = func(_ context.Context, request *essbasic.DescribeFlowDetailInfoRequest) (*essbasic.DescribeFlowDetailInfoResponse, error) {
		response := essbasic.NewDescribeFlowDetailInfoResponse()
		response.Response = &essbasic.DescribeFlowDetailInfoResponseParams{
			ApplicationId:           common.StringPtr("app-id"),
			ProxyOrganizationOpenId: common.StringPtr("organization-id"),
			FlowInfo: []*essbasic.FlowDetailInfo{{
				FlowId: common.StringPtr(testFlowID), FlowStatus: common.StringPtr("PART"),
			}},
			RequestId: common.StringPtr("request-id"),
		}
		if len(request.FlowIds) != 1 || stringValue(request.FlowIds[0]) != testFlowID {
			t.Fatalf("unexpected query request: %+v", request)
		}
		return response, nil
	}
	client := testClient(t, api, nil, CapabilityFlowQuery)
	observedAt := time.Date(2026, 8, 14, 1, 2, 3, 0, time.UTC)
	client.now = func() time.Time { return observedAt }
	result, err := client.Query(context.Background(), suppliercallback.QueryRequest{
		ExternalReference: []byte(testFlowID),
	})
	if err != nil || result.Status != suppliercallback.StatusRunning ||
		result.SupplierObservedAt != observedAt || len(result.EvidenceFrame) == 0 {
		t.Fatalf("unexpected query result: %+v err=%v", result, err)
	}
}

func TestQueryRejectsCrossSubjectResponse(t *testing.T) {
	api := inertAPI()
	api.flows = func(_ context.Context, _ *essbasic.DescribeFlowDetailInfoRequest) (*essbasic.DescribeFlowDetailInfoResponse, error) {
		response := essbasic.NewDescribeFlowDetailInfoResponse()
		response.Response = &essbasic.DescribeFlowDetailInfoResponseParams{
			ApplicationId:           common.StringPtr("app-id"),
			ProxyOrganizationOpenId: common.StringPtr("another-organization"),
			FlowInfo: []*essbasic.FlowDetailInfo{{
				FlowId: common.StringPtr(testFlowID), FlowStatus: common.StringPtr("PART"),
			}},
			RequestId: common.StringPtr("request-id"),
		}
		return response, nil
	}
	client := testClient(t, api, nil, CapabilityFlowQuery)
	_, err := client.Query(context.Background(), suppliercallback.QueryRequest{
		ExternalReference: []byte(testFlowID),
	})
	var safe *Error
	if !errors.As(err, &safe) || safe.Code != ErrorMalformedResponse {
		t.Fatalf("cross-subject response was not rejected safely: %v", err)
	}
}

func TestCreateSignURLFixesThirtyMinuteLifetime(t *testing.T) {
	api := inertAPI()
	api.signURL = func(_ context.Context, request *essbasic.ChannelCreateFlowSignUrlRequest) (*essbasic.ChannelCreateFlowSignUrlResponse, error) {
		if request.ExpiredOn == nil || *request.ExpiredOn != signURLLifetimeSeconds ||
			request.UrlType == nil || *request.UrlType != 0 {
			t.Fatalf("unexpected sign URL lifetime: %+v", request)
		}
		response := essbasic.NewChannelCreateFlowSignUrlResponse()
		response.Response = &essbasic.ChannelCreateFlowSignUrlResponseParams{
			FlowApproverUrlInfos: []*essbasic.FlowApproverUrlInfo{{
				SignUrl: common.StringPtr("https://ess.tencent.example/sign"),
			}},
			RequestId: common.StringPtr("request-id"),
		}
		return response, nil
	}
	client := testClient(t, api, nil, CapabilitySignURL)
	_, err := client.CreateSignURL(
		context.Background(), testFlowID, []*essbasic.FlowApproverInfo{{}},
		"https://customer.example.com/contracts/return",
	)
	if err != nil {
		t.Fatalf("CreateSignURL returned error: %v", err)
	}
	if _, err = client.CreateSignURL(
		context.Background(), testFlowID, []*essbasic.FlowApproverInfo{{}},
		"http://customer.example.com/contracts/return",
	); err == nil {
		t.Fatal("insecure jump URL was accepted")
	}
}

func TestSafeErrorDoesNotExposeTencentFreeText(t *testing.T) {
	providerError := &tencenterrors.TencentCloudSDKError{
		Code: "AuthFailure.SecretIdNotFound", Message: "raw secret diagnostic", RequestId: "request-id",
	}
	safe := safeError(providerError, true)
	if safe.Code != ErrorCredentialRejected || safe.RequestID != "request-id" ||
		safe.Error() != string(ErrorCredentialRejected) {
		t.Fatalf("unexpected safe error: %+v", safe)
	}
}

func TestDescribeTemplatePreviewUsesExactOfficialRead(t *testing.T) {
	api := inertAPI()
	api.templates = func(_ context.Context, request *essbasic.DescribeTemplatesRequest) (*essbasic.DescribeTemplatesResponse, error) {
		if !validAgent(request.Agent) || request.TemplateId == nil || *request.TemplateId != testFlowID ||
			request.ContentType == nil || *request.ContentType != 1 ||
			request.WithPreviewUrl == nil || !*request.WithPreviewUrl {
			t.Fatalf("unexpected template preview request: %+v", request)
		}
		response := essbasic.NewDescribeTemplatesResponse()
		response.Response = &essbasic.DescribeTemplatesResponseParams{
			RequestId: common.StringPtr("request-id"),
			Templates: []*essbasic.TemplateInfo{{
				TemplateId: common.StringPtr(testFlowID),
				PreviewUrl: common.StringPtr("https://ess.tencent.example/template-preview"),
			}},
		}
		return response, nil
	}
	client := testClient(t, api, nil, CapabilityTemplateRead)
	previewURL, err := client.DescribeTemplatePreview(context.Background(), testFlowID)
	if err != nil || previewURL != "https://ess.tencent.example/template-preview" {
		t.Fatalf("unexpected template preview: url=%q err=%v", previewURL, err)
	}
}

func TestDescribeTemplatePreviewRejectsUnsafeOfficialResponse(t *testing.T) {
	api := inertAPI()
	api.templates = func(context.Context, *essbasic.DescribeTemplatesRequest) (*essbasic.DescribeTemplatesResponse, error) {
		response := essbasic.NewDescribeTemplatesResponse()
		response.Response = &essbasic.DescribeTemplatesResponseParams{
			RequestId: common.StringPtr("request-id"),
			Templates: []*essbasic.TemplateInfo{{
				TemplateId: common.StringPtr(testFlowID),
				PreviewUrl: common.StringPtr("http://unsafe.example/template-preview"),
			}},
		}
		return response, nil
	}
	client := testClient(t, api, nil, CapabilityTemplateRead)
	_, err := client.DescribeTemplatePreview(context.Background(), testFlowID)
	var safe *Error
	if !errors.As(err, &safe) || safe.Code != ErrorMalformedResponse {
		t.Fatalf("unsafe official preview response was accepted: %v", err)
	}
}

func inertAPI() *fakeOfficialAPI {
	unexpected := func() error { return errors.New("unexpected Tencent API call") }
	return &fakeOfficialAPI{
		templates: func(context.Context, *essbasic.DescribeTemplatesRequest) (*essbasic.DescribeTemplatesResponse, error) {
			return nil, unexpected()
		},
		create: func(context.Context, *essbasic.CreateFlowsByTemplatesRequest) (*essbasic.CreateFlowsByTemplatesResponse, error) {
			return nil, unexpected()
		},
		release: func(context.Context, *essbasic.ChannelCreateReleaseFlowRequest) (*essbasic.ChannelCreateReleaseFlowResponse, error) {
			return nil, unexpected()
		},
		signURL: func(context.Context, *essbasic.ChannelCreateFlowSignUrlRequest) (*essbasic.ChannelCreateFlowSignUrlResponse, error) {
			return nil, unexpected()
		},
		flows: func(context.Context, *essbasic.DescribeFlowDetailInfoRequest) (*essbasic.DescribeFlowDetailInfoResponse, error) {
			return nil, unexpected()
		},
		download: func(context.Context, *essbasic.GetDownloadFlowUrlRequest) (*essbasic.GetDownloadFlowUrlResponse, error) {
			return nil, unexpected()
		},
		resources: func(context.Context, *essbasic.DescribeResourceUrlsByFlowsRequest) (*essbasic.DescribeResourceUrlsByFlowsResponse, error) {
			return nil, unexpected()
		},
		createReport: func(context.Context, *essbasic.CreateChannelFlowEvidenceReportRequest) (*essbasic.CreateChannelFlowEvidenceReportResponse, error) {
			return nil, unexpected()
		},
		describeReport: func(context.Context, *essbasic.DescribeChannelFlowEvidenceReportRequest) (*essbasic.DescribeChannelFlowEvidenceReportResponse, error) {
			return nil, unexpected()
		},
		autoSign: func(context.Context, *essbasic.DescribeExtendedServiceAuthInfoRequest) (*essbasic.DescribeExtendedServiceAuthInfoResponse, error) {
			return nil, unexpected()
		},
		usage: func(context.Context, *essbasic.ChannelDescribeBillUsageDetailRequest) (*essbasic.ChannelDescribeBillUsageDetailResponse, error) {
			return nil, unexpected()
		},
		accountBill: func(context.Context, *essbasic.ChannelDescribeAccountBillDetailRequest) (*essbasic.ChannelDescribeAccountBillDetailResponse, error) {
			return nil, unexpected()
		},
	}
}

func testClient(t *testing.T, api officialAPI, guard writeGuard, capabilities ...Capability) *Client {
	t.Helper()
	config := Config{
		APIVersion: APIVersion, SecretID: "secret-id", SecretKey: "secret-key",
		AppID: "app-id", Capabilities: capabilities,
	}
	credential := common.NewCredential(config.SecretID, config.SecretKey)
	subjects := &fakeSubjectSource{payload: []byte(
		`{"proxyOrganizationOpenId":"organization-id","proxyOperatorOpenId":"operator-id"}`,
	)}
	client, err := newClient(config, api, credential, subjects, guard)
	if err != nil {
		t.Fatalf("newClient returned error: %v", err)
	}
	t.Cleanup(client.Close)
	return client
}
