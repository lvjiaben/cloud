package tencentess

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net"
	"testing"
	"time"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	essbasic "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/essbasic/v20210526"

	"lina-platform/idc-compliance-gateway/internal/supplierguard"
)

type artifactDownloaderStub struct {
	url     string
	content []byte
	err     error
}

func (s *artifactDownloaderStub) DownloadPDF(_ context.Context, rawURL string) ([]byte, error) {
	s.url = rawURL
	if s.err != nil {
		return nil, s.err
	}
	return append([]byte(nil), s.content...), nil
}

func TestFetchFinalPDFConsumesExactOfficialResource(t *testing.T) {
	api := inertAPI()
	api.resources = func(
		_ context.Context,
		request *essbasic.DescribeResourceUrlsByFlowsRequest,
	) (*essbasic.DescribeResourceUrlsByFlowsResponse, error) {
		if len(request.FlowIds) != 1 || stringValue(request.FlowIds[0]) != testFlowID || !validAgent(request.Agent) {
			t.Fatalf("unexpected final PDF request: %+v", request)
		}
		response := essbasic.NewDescribeResourceUrlsByFlowsResponse()
		response.Response = &essbasic.DescribeResourceUrlsByFlowsResponseParams{
			FlowResourceUrlInfos: []*essbasic.FlowResourceUrlInfo{{
				FlowId: common.StringPtr(testFlowID),
				ResourceUrlInfos: []*essbasic.ResourceUrlInfo{{
					Name: common.StringPtr("contract.pdf"), Type: common.StringPtr("PDF"),
					Url: common.StringPtr("https://file.test.ess.tencent.cn/contract.pdf?token=short-lived"),
				}},
			}},
			ErrorMessages: []*string{common.StringPtr("")}, RequestId: common.StringPtr("request-id"),
		}
		return response, nil
	}
	client := testClient(t, api, nil, CapabilityDownload)
	downloader := &artifactDownloaderStub{content: []byte("%PDF-1.7\nverified")}
	client.downloader = downloader
	observedAt := time.Date(2026, 8, 17, 1, 2, 3, 0, time.UTC)
	client.now = func() time.Time { return observedAt }
	document, err := client.FetchFinalPDF(t.Context(), testFlowID)
	if err != nil {
		t.Fatalf("FetchFinalPDF returned error: %v", err)
	}
	digest := sha256.Sum256(downloader.content)
	if string(document.Content) != string(downloader.content) ||
		document.SHA256 != hex.EncodeToString(digest[:]) || document.ObservedAt != observedAt ||
		downloader.url != "https://file.test.ess.tencent.cn/contract.pdf?token=short-lived" {
		t.Fatalf("unexpected final PDF result: document=%+v url=%q", document, downloader.url)
	}
}

func TestCreateEvidenceReportUsesPaidGuardOnce(t *testing.T) {
	api := inertAPI()
	api.createReport = func(
		_ context.Context,
		request *essbasic.CreateChannelFlowEvidenceReportRequest,
	) (*essbasic.CreateChannelFlowEvidenceReportResponse, error) {
		if !validAgent(request.Agent) || stringValue(request.FlowId) != testFlowID ||
			request.ReportType == nil || *request.ReportType != reportTypeContractSigning {
			t.Fatalf("unexpected evidence report request: %+v", request)
		}
		response := essbasic.NewCreateChannelFlowEvidenceReportResponse()
		response.Response = &essbasic.CreateChannelFlowEvidenceReportResponseParams{
			ReportId: common.StringPtr("report-id"), Status: common.StringPtr(reportStatusExecuting),
			RequestId: common.StringPtr("request-id"),
		}
		return response, nil
	}
	guard := &fakeWriteGuard{lease: supplierguard.Lease{
		AttemptNo: "lwa_0123456789abcdef0123456789abcdef",
		RouteNo:   "lwr_0123456789abcdef0123456789abcdef", CallSupplier: true,
	}}
	client := testClient(t, api, guard, CapabilityEvidenceReport)
	result, err := client.CreateEvidenceReport(
		t.Context(), "lbr_0123456789abcdef0123456789abcdef", testFlowID,
	)
	if err != nil || result.ReportID != "report-id" || result.Status != reportStatusExecuting ||
		guard.acquired.OperationScope != OperationEvidenceReportCreate ||
		guard.observation.Result != supplierguard.ResultSucceeded {
		t.Fatalf("unexpected guarded report result=%+v guard=%+v err=%v", result, guard, err)
	}
}

func TestFetchEvidenceReportConsumesFiveMinuteURL(t *testing.T) {
	api := inertAPI()
	api.describeReport = func(
		_ context.Context,
		request *essbasic.DescribeChannelFlowEvidenceReportRequest,
	) (*essbasic.DescribeChannelFlowEvidenceReportResponse, error) {
		if stringValue(request.ReportId) != "report-id" || request.ReportType == nil ||
			*request.ReportType != reportTypeContractSigning || !validAgent(request.Agent) {
			t.Fatalf("unexpected report query: %+v", request)
		}
		response := essbasic.NewDescribeChannelFlowEvidenceReportResponse()
		response.Response = &essbasic.DescribeChannelFlowEvidenceReportResponseParams{
			Status:    common.StringPtr(reportStatusSuccess),
			ReportUrl: common.StringPtr("https://file.test.ess.tencent.cn/report.pdf?token=five-minute"),
			RequestId: common.StringPtr("request-id"),
		}
		return response, nil
	}
	client := testClient(t, api, &fakeWriteGuard{}, CapabilityEvidenceReport)
	downloader := &artifactDownloaderStub{content: []byte("%PDF-1.7\nreport")}
	client.downloader = downloader
	result, err := client.FetchEvidenceReport(t.Context(), "report-id")
	if err != nil || result.Status != reportStatusSuccess || result.Document == nil ||
		string(result.Document.Content) != string(downloader.content) || downloader.url == "" {
		t.Fatalf("unexpected report result=%+v url=%q err=%v", result, downloader.url, err)
	}
}

func TestArtifactURLRejectsUnsafeDestinations(t *testing.T) {
	for _, candidate := range []string{
		"http://file.test.ess.tencent.cn/file.pdf",
		"https://127.0.0.1/file.pdf",
		"https://user:password@file.test.ess.tencent.cn/file.pdf",
		"https://file.test.ess.tencent.cn/file.pdf#secret",
		"https://file.test.ess.tencent.cn:8443/file.pdf",
	} {
		if safeArtifactURL(candidate) {
			t.Fatalf("unsafe artifact URL was accepted: %q", candidate)
		}
	}
}

func TestArtifactAddressRejectsReservedNetworks(t *testing.T) {
	for _, candidate := range []string{
		"100.64.0.1", "192.0.2.1", "198.18.0.1", "203.0.113.1", "2001:db8::1",
	} {
		if publicArtifactIP(net.ParseIP(candidate)) {
			t.Fatalf("reserved artifact address was accepted: %q", candidate)
		}
	}
	for _, candidate := range []string{"1.1.1.1", "2606:4700:4700::1111"} {
		if !publicArtifactIP(net.ParseIP(candidate)) {
			t.Fatalf("public artifact address was rejected: %q", candidate)
		}
	}
}
