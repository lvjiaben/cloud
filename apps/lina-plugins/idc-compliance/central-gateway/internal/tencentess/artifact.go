// This file fetches short-lived Tencent contract resources immediately and
// never exposes or persists their URLs outside the installation-local gateway.
package tencentess

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"path"
	"strings"
	"time"

	essbasic "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/essbasic/v20210526"

	"lina-platform/idc-compliance-gateway/internal/supplierguard"
)

const (
	maximumContractArtifactBytes int64 = 32 << 20
	reportTypeContractSigning    int64 = 0
	reportStatusExecuting              = "EvidenceStatusExecuting"
	reportStatusSuccess                = "EvidenceStatusSuccess"
	reportStatusFailed                 = "EvidenceStatusFailed"

	// OperationEvidenceReportCreate is the paid, guarded signing-report request.
	OperationEvidenceReportCreate = "tencent.contract.evidence-report.create"
)

var artifactDeniedPrefixes = mustArtifactPrefixes(
	"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16",
	"172.16.0.0/12", "192.0.0.0/24", "192.0.2.0/24", "192.31.196.0/24", "192.52.193.0/24",
	"192.88.99.0/24", "192.168.0.0/16", "192.175.48.0/24", "198.18.0.0/15",
	"198.51.100.0/24", "203.0.113.0/24", "224.0.0.0/4", "240.0.0.0/4",
	"::/128", "::1/128", "64:ff9b::/96", "64:ff9b:1::/48", "100::/64", "100:0:0:1::/64",
	"2001::/23", "2001:db8::/32", "2002::/16", "2620:4f:8000::/48", "3fff::/20", "5f00::/16",
	"fc00::/7", "fe80::/10", "ff00::/8",
)

type artifactDownloader interface {
	DownloadPDF(context.Context, string) ([]byte, error)
}

type secureArtifactDownloader struct {
	client *http.Client
}

// ArtifactDocument is one bounded PDF fetched before its supplier URL expires.
type ArtifactDocument struct {
	Content    []byte
	SHA256     string
	ObservedAt time.Time
}

// EvidenceReportCreateResult keeps only the guarded local route and the
// short-lived in-memory report identifier needed by the local encrypted store.
type EvidenceReportCreateResult struct {
	RouteNo    string
	ReportID   string
	Status     string
	ObservedAt time.Time
	Recovered  bool
}

// EvidenceReportResult is a read-only report status and optional ready PDF.
type EvidenceReportResult struct {
	Status     string
	Document   *ArtifactDocument
	ObservedAt time.Time
}

func newArtifactDownloader() artifactDownloader {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil || port != "443" || net.ParseIP(host) != nil {
				return nil, errors.New("tencent artifact address is invalid")
			}
			addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil || len(addresses) == 0 || len(addresses) > 16 {
				return nil, errors.New("tencent artifact host cannot be resolved safely")
			}
			for _, candidate := range addresses {
				if !publicArtifactIP(candidate.IP) {
					return nil, errors.New("tencent artifact host resolved to a non-public address")
				}
			}
			var lastErr error
			for _, candidate := range addresses {
				connection, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(candidate.IP.String(), port))
				if dialErr == nil {
					return connection, nil
				}
				lastErr = dialErr
			}
			return nil, lastErr
		},
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		DisableCompression:    true,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          4,
		MaxIdleConnsPerHost:   2,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
	}
	return &secureArtifactDownloader{client: &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return errors.New("tencent artifact redirects are disabled")
		},
	}}
}

func publicArtifactIP(ip net.IP) bool {
	address, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	address = address.Unmap()
	if !address.IsGlobalUnicast() || address.IsPrivate() || address.IsLoopback() ||
		address.IsLinkLocalUnicast() || address.IsLinkLocalMulticast() ||
		address.IsMulticast() || address.IsUnspecified() {
		return false
	}
	for _, prefix := range artifactDeniedPrefixes {
		if prefix.Contains(address) {
			return false
		}
	}
	return true
}

func mustArtifactPrefixes(values ...string) []netip.Prefix {
	result := make([]netip.Prefix, len(values))
	for index, value := range values {
		result[index] = netip.MustParsePrefix(value)
	}
	return result
}

func (d *secureArtifactDownloader) DownloadPDF(ctx context.Context, rawURL string) ([]byte, error) {
	if d == nil || d.client == nil || ctx == nil || ctx.Err() != nil || !safeArtifactURL(rawURL) {
		return nil, &Error{Code: ErrorInvalidInput}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, &Error{Code: ErrorInvalidInput, cause: err}
	}
	request.Header.Set("Accept", "application/pdf, application/octet-stream;q=0.8")
	response, err := d.client.Do(request)
	if err != nil {
		return nil, safeError(err, false)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || response.ContentLength > maximumContractArtifactBytes {
		return nil, &Error{Code: ErrorServiceUnavailable}
	}
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(response.Header.Get("Content-Type"), ";")[0]))
	if contentType != "" && contentType != "application/pdf" && contentType != "application/octet-stream" {
		return nil, &Error{Code: ErrorMalformedResponse}
	}
	content, err := io.ReadAll(io.LimitReader(response.Body, maximumContractArtifactBytes+1))
	if err != nil || len(content) < 5 || int64(len(content)) > maximumContractArtifactBytes ||
		string(content[:5]) != "%PDF-" {
		clearBytes(content)
		return nil, &Error{Code: ErrorMalformedResponse, cause: err}
	}
	return content, nil
}

func safeArtifactURL(raw string) bool {
	if len(raw) < 1 || len(raw) > 2000 || strings.ContainsAny(raw, "\r\n\t\x00") {
		return false
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.User != nil || parsed.Fragment != "" ||
		parsed.Hostname() == "" || net.ParseIP(parsed.Hostname()) != nil {
		return false
	}
	return parsed.Port() == "" || parsed.Port() == "443"
}

// FetchFinalPDF obtains exactly one official PDF URL and consumes it immediately.
func (c *Client) FetchFinalPDF(ctx context.Context, flowID string) (*ArtifactDocument, error) {
	if !c.canCall(ctx, CapabilityDownload) || !flowIDPattern.MatchString(flowID) || c.downloader == nil {
		return nil, &Error{Code: ErrorInvalidInput}
	}
	request := essbasic.NewDescribeResourceUrlsByFlowsRequest()
	request.FlowIds = []*string{&flowID}
	var response *essbasic.DescribeResourceUrlsByFlowsResponse
	var callErr error
	subjectErr := c.withAgent(ctx, func(agent *essbasic.Agent) error {
		request.Agent = agent
		response, callErr = c.api.DescribeResourceUrlsByFlowsWithContext(ctx, request)
		return callErr
	})
	if subjectErr != nil {
		return nil, providerCallError(subjectErr, callErr, false)
	}
	resourceURL, err := exactFinalPDFURL(response, flowID)
	if err != nil {
		return nil, err
	}
	content, err := c.downloader.DownloadPDF(ctx, resourceURL)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(content)
	return &ArtifactDocument{
		Content: content, SHA256: hex.EncodeToString(digest[:]), ObservedAt: c.now().UTC(),
	}, nil
}

func exactFinalPDFURL(response *essbasic.DescribeResourceUrlsByFlowsResponse, flowID string) (string, error) {
	if response == nil || response.Response == nil || response.Response.RequestId == nil ||
		len(response.Response.FlowResourceUrlInfos) != 1 || len(response.Response.ErrorMessages) != 1 ||
		strings.TrimSpace(stringValue(response.Response.ErrorMessages[0])) != "" {
		return "", &Error{Code: ErrorMalformedResponse}
	}
	flow := response.Response.FlowResourceUrlInfos[0]
	if flow == nil || stringValue(flow.FlowId) != flowID || len(flow.ResourceUrlInfos) != 1 {
		return "", &Error{Code: ErrorMalformedResponse}
	}
	resource := flow.ResourceUrlInfos[0]
	if resource == nil {
		return "", &Error{Code: ErrorMalformedResponse}
	}
	name := strings.TrimSpace(stringValue(resource.Name))
	resourceURL := strings.TrimSpace(stringValue(resource.Url))
	if stringValue(resource.Type) != "PDF" || name == "" ||
		!strings.EqualFold(path.Ext(name), ".pdf") || !safeArtifactURL(resourceURL) {
		return "", &Error{Code: ErrorMalformedResponse}
	}
	return resourceURL, nil
}

// CreateEvidenceReport submits one buyer-paid report request through the shared write guard.
func (c *Client) CreateEvidenceReport(
	ctx context.Context,
	requestNo string,
	flowID string,
) (EvidenceReportCreateResult, error) {
	if !c.canCall(ctx, CapabilityEvidenceReport) || c.guard == nil || requestNo == "" ||
		!flowIDPattern.MatchString(flowID) {
		return EvidenceReportCreateResult{}, &Error{Code: ErrorInvalidInput}
	}
	if err := c.admitNewCharge(ctx); err != nil {
		return EvidenceReportCreateResult{}, err
	}
	var result EvidenceReportCreateResult
	var operationErr error
	subjectErr := c.withAgent(ctx, func(agent *essbasic.Agent) error {
		lease, err := c.guard.Acquire(ctx, supplierguard.Admission{
			RequestNo: requestNo, Provider: providerCode, OperationScope: OperationEvidenceReportCreate,
		})
		if err != nil {
			operationErr = guardError(err)
			return operationErr
		}
		result.RouteNo = lease.RouteNo
		result.Recovered = lease.Recovered
		if !lease.CallSupplier {
			operationErr = recoveredWriteError(lease)
			return operationErr
		}
		started := c.now()
		request := essbasic.NewCreateChannelFlowEvidenceReportRequest()
		request.Agent = agent
		request.FlowId = &flowID
		request.ReportType = commonInt64(reportTypeContractSigning)
		response, callErr := c.api.CreateChannelFlowEvidenceReportWithContext(ctx, request)
		if callErr != nil {
			operationErr = c.finishFailedWrite(ctx, lease, started, safeError(callErr, true))
			return operationErr
		}
		if response == nil || response.Response == nil || response.Response.RequestId == nil ||
			!validOpaque(stringValue(response.Response.ReportId), 1, 128) ||
			!validReportStatus(stringValue(response.Response.Status)) {
			operationErr = c.finishFailedWrite(ctx, lease, started, &Error{Code: ErrorResultUnknown})
			return operationErr
		}
		result.ReportID = stringValue(response.Response.ReportId)
		result.Status = stringValue(response.Response.Status)
		result.ObservedAt = c.now().UTC()
		if result.Status == reportStatusFailed {
			operationErr = c.finishFailedWrite(ctx, lease, started, &Error{Code: ErrorRejected})
			return operationErr
		}
		if err = c.observeWrite(ctx, lease, supplierguard.ResultSucceeded, "tencent_ok", started); err != nil {
			operationErr = &Error{Code: ErrorResultUnknown, cause: err}
			return operationErr
		}
		return nil
	})
	if subjectErr != nil {
		if operationErr != nil {
			return result, operationErr
		}
		return result, &Error{Code: ErrorSubjectUnavailable, cause: subjectErr}
	}
	return result, nil
}

// FetchEvidenceReport queries one existing report and consumes its five-minute URL immediately.
func (c *Client) FetchEvidenceReport(ctx context.Context, reportID string) (EvidenceReportResult, error) {
	if !c.canCall(ctx, CapabilityEvidenceReport) || !validOpaque(reportID, 1, 128) || c.downloader == nil {
		return EvidenceReportResult{}, &Error{Code: ErrorInvalidInput}
	}
	request := essbasic.NewDescribeChannelFlowEvidenceReportRequest()
	request.ReportId = &reportID
	request.ReportType = commonInt64(reportTypeContractSigning)
	var response *essbasic.DescribeChannelFlowEvidenceReportResponse
	var callErr error
	subjectErr := c.withAgent(ctx, func(agent *essbasic.Agent) error {
		request.Agent = agent
		response, callErr = c.api.DescribeChannelFlowEvidenceReportWithContext(ctx, request)
		return callErr
	})
	if subjectErr != nil {
		return EvidenceReportResult{}, providerCallError(subjectErr, callErr, false)
	}
	if response == nil || response.Response == nil || response.Response.RequestId == nil ||
		!validReportStatus(stringValue(response.Response.Status)) {
		return EvidenceReportResult{}, &Error{Code: ErrorMalformedResponse}
	}
	observedAt := c.now().UTC()
	status := stringValue(response.Response.Status)
	result := EvidenceReportResult{Status: status, ObservedAt: observedAt}
	if status != reportStatusSuccess {
		if response.Response.ReportUrl != nil && strings.TrimSpace(*response.Response.ReportUrl) != "" {
			return EvidenceReportResult{}, &Error{Code: ErrorMalformedResponse}
		}
		return result, nil
	}
	reportURL := strings.TrimSpace(stringValue(response.Response.ReportUrl))
	if !safeArtifactURL(reportURL) {
		return EvidenceReportResult{}, &Error{Code: ErrorMalformedResponse}
	}
	content, err := c.downloader.DownloadPDF(ctx, reportURL)
	if err != nil {
		return EvidenceReportResult{}, err
	}
	digest := sha256.Sum256(content)
	result.Document = &ArtifactDocument{
		Content: content, SHA256: hex.EncodeToString(digest[:]), ObservedAt: observedAt,
	}
	return result, nil
}

func validReportStatus(status string) bool {
	return status == reportStatusExecuting || status == reportStatusSuccess || status == reportStatusFailed
}

func commonInt64(value int64) *int64 {
	return &value
}
