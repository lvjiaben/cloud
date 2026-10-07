// This file implements the fixed Tencent SDK client, guarded writes, bounded
// reads, reconciliation queries, and secret-safe response validation.

package tencentess

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	essbasic "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/essbasic/v20210526"

	"lina-platform/idc-compliance-gateway/internal/suppliercallback"
	"lina-platform/idc-compliance-gateway/internal/supplierguard"
)

const (
	providerCode           = supplierguard.ProviderTencentESSBasic
	requestTimeoutSeconds  = 30
	signURLLifetimeSeconds = int64(1800)
	maximumEvidenceBytes   = 256 << 10

	// OperationFlowCreate is the guarded operation scope for contract creation.
	OperationFlowCreate = "tencent.contract.create"
	// OperationFlowRelease is the guarded operation scope for a release agreement.
	OperationFlowRelease = "tencent.contract.release"
	// OperationFlowPreview is the guarded operation scope for contract previews.
	OperationFlowPreview = "tencent.contract.preview"
)

var flowIDPattern = regexp.MustCompile(`^[A-Za-z0-9]{32}$`)

type officialAPI interface {
	DescribeTemplatesWithContext(context.Context, *essbasic.DescribeTemplatesRequest) (*essbasic.DescribeTemplatesResponse, error)
	CreateFlowsByTemplatesWithContext(context.Context, *essbasic.CreateFlowsByTemplatesRequest) (*essbasic.CreateFlowsByTemplatesResponse, error)
	ChannelCreateReleaseFlowWithContext(context.Context, *essbasic.ChannelCreateReleaseFlowRequest) (*essbasic.ChannelCreateReleaseFlowResponse, error)
	ChannelCreateFlowSignUrlWithContext(context.Context, *essbasic.ChannelCreateFlowSignUrlRequest) (*essbasic.ChannelCreateFlowSignUrlResponse, error)
	DescribeFlowDetailInfoWithContext(context.Context, *essbasic.DescribeFlowDetailInfoRequest) (*essbasic.DescribeFlowDetailInfoResponse, error)
	GetDownloadFlowUrlWithContext(context.Context, *essbasic.GetDownloadFlowUrlRequest) (*essbasic.GetDownloadFlowUrlResponse, error)
	DescribeResourceUrlsByFlowsWithContext(context.Context, *essbasic.DescribeResourceUrlsByFlowsRequest) (*essbasic.DescribeResourceUrlsByFlowsResponse, error)
	CreateChannelFlowEvidenceReportWithContext(context.Context, *essbasic.CreateChannelFlowEvidenceReportRequest) (*essbasic.CreateChannelFlowEvidenceReportResponse, error)
	DescribeChannelFlowEvidenceReportWithContext(context.Context, *essbasic.DescribeChannelFlowEvidenceReportRequest) (*essbasic.DescribeChannelFlowEvidenceReportResponse, error)
	DescribeExtendedServiceAuthInfoWithContext(context.Context, *essbasic.DescribeExtendedServiceAuthInfoRequest) (*essbasic.DescribeExtendedServiceAuthInfoResponse, error)
	ChannelDescribeBillUsageDetailWithContext(context.Context, *essbasic.ChannelDescribeBillUsageDetailRequest) (*essbasic.ChannelDescribeBillUsageDetailResponse, error)
	ChannelDescribeAccountBillDetailWithContext(context.Context, *essbasic.ChannelDescribeAccountBillDetailRequest) (*essbasic.ChannelDescribeAccountBillDetailResponse, error)
}

type onboardingAPI interface {
	CreateConsoleLoginUrlWithContext(context.Context, *essbasic.CreateConsoleLoginUrlRequest) (*essbasic.CreateConsoleLoginUrlResponse, error)
}

type writeGuard interface {
	Acquire(context.Context, supplierguard.Admission) (supplierguard.Lease, error)
	Observe(context.Context, supplierguard.Observation) (supplierguard.Lease, error)
}

type referenceBinder interface {
	BindExternalReference(context.Context, suppliercallback.ReferenceBinding) (int64, error)
}

type chargeAdmission interface {
	AdmitNewCharge(context.Context, string) error
}

// Client is the fixed-endpoint Tencent eSign adapter for one buyer installation.
type Client struct {
	api          officialAPI
	onboarding   onboardingAPI
	credential   *common.Credential
	appID        string
	subjects     subjectSource
	capabilities map[Capability]struct{}
	guard        writeGuard
	chargeGate   chargeAdmission
	references   referenceBinder
	downloader   artifactDownloader
	now          func() time.Time
}

// New constructs the official SDK client without proxy, endpoint, or retry overrides.
func New(config Config, subjects subjectSource, guard writeGuard) (*Client, error) {
	if err := config.Validate(); err != nil {
		return nil, &Error{Code: ErrorNotConfigured, cause: err}
	}
	credential := common.NewCredential(config.SecretID, config.SecretKey)
	clientProfile := officialClientProfile()
	api, err := essbasic.NewClient(credential, "", clientProfile)
	if err != nil {
		credential.SecretId = ""
		credential.SecretKey = ""
		return nil, &Error{Code: ErrorNotConfigured, cause: err}
	}
	return newClient(config, api, credential, subjects, guard)
}

func newClient(
	config Config,
	api officialAPI,
	credential *common.Credential,
	subjects subjectSource,
	guard writeGuard,
) (*Client, error) {
	if err := config.Validate(); err != nil || api == nil || credential == nil || subjects == nil {
		return nil, &Error{Code: ErrorNotConfigured, cause: err}
	}
	if (config.HasCapability(CapabilityFlowCreate) || config.HasCapability(CapabilityFlowRelease) ||
		config.HasCapability(CapabilityEvidenceReport)) && guard == nil {
		return nil, &Error{Code: ErrorNotConfigured, cause: errors.New("write protection is unavailable")}
	}
	capabilities := make(map[Capability]struct{}, len(config.Capabilities))
	for _, capability := range config.Capabilities {
		capabilities[capability] = struct{}{}
	}
	return &Client{
		api: api, credential: credential, appID: config.AppID, subjects: subjects,
		onboarding: func() onboardingAPI {
			value, _ := api.(onboardingAPI)
			return value
		}(),
		capabilities: capabilities, guard: guard, downloader: newArtifactDownloader(),
		now: time.Now,
	}, nil
}

// SetReferenceBinder completes the startup-only cycle between the guarded
// writer and the read-only reconciliation service. It can succeed only once.
func (c *Client) SetReferenceBinder(references referenceBinder) error {
	if !c.Configured() || references == nil || c.references != nil {
		return &Error{Code: ErrorNotConfigured}
	}
	c.references = references
	return nil
}

// SetChargeAdmission attaches the installation-local supplier-plan gate. The
// gate is optional for isolated adapter tests and can be attached only once at
// startup after the local usage service has been constructed.
func (c *Client) SetChargeAdmission(gate chargeAdmission) error {
	if !c.Configured() || gate == nil || c.chargeGate != nil {
		return &Error{Code: ErrorNotConfigured}
	}
	c.chargeGate = gate
	return nil
}

func officialClientProfile() *profile.ClientProfile {
	result := profile.NewClientProfile()
	result.SignMethod = "TC3-HMAC-SHA256"
	result.Language = "zh-CN"
	result.Debug = false
	result.DisableRegionBreaker = true
	result.BackupEndpoint = ""
	result.NetworkFailureMaxRetries = 0
	result.RateLimitExceededMaxRetries = 0
	result.UnsafeRetryOnConnectionFailure = false
	result.HttpProfile.ReqMethod = "POST"
	result.HttpProfile.ReqTimeout = requestTimeoutSeconds
	result.HttpProfile.Scheme = "HTTPS"
	result.HttpProfile.Endpoint = OfficialEndpoint
	result.HttpProfile.RootDomain = ""
	result.HttpProfile.Proxy = ""
	result.HttpProfile.ApigwEndpoint = ""
	return result
}

// Configured reports whether the startup-frozen SDK and buyer identity are usable locally.
func (c *Client) Configured() bool {
	return c != nil && c.api != nil && c.credential != nil && c.credential.SecretId != "" &&
		c.credential.SecretKey != "" && validOpaque(c.appID, 1, 64) && c.subjects != nil &&
		len(c.capabilities) > 0
}

// HasCapability reports whether one ability was explicitly enabled by this installation.
func (c *Client) HasCapability(capability Capability) bool {
	if !c.Configured() {
		return false
	}
	_, ok := c.capabilities[capability]
	return ok
}

// Close clears the SDK credential and copied application identifiers.
func (c *Client) Close() {
	if c == nil {
		return
	}
	if c.credential != nil {
		c.credential.SecretId = ""
		c.credential.SecretKey = ""
		c.credential.Token = ""
	}
	c.appID = ""
	c.subjects = nil
	clear(c.capabilities)
	c.capabilities = nil
	c.api = nil
	c.onboarding = nil
	c.downloader = nil
}

// DescribeTemplates performs one bounded official template query.
func (c *Client) DescribeTemplates(
	ctx context.Context,
	limit uint64,
	offset uint64,
) (*essbasic.DescribeTemplatesResponse, error) {
	if !c.canCall(ctx, CapabilityTemplateRead) || limit < 1 || limit > 100 || offset > 20000 {
		return nil, &Error{Code: ErrorInvalidInput}
	}
	// Full component and recipient metadata is required to prove the local
	// customer/merchant signing-position enablement gate. The local gateway
	// projects only counts and never returns the supplier component payload.
	contentType := int64(0)
	queryAllComponents := true
	request := essbasic.NewDescribeTemplatesRequest()
	request.ContentType = &contentType
	request.QueryAllComponents = &queryAllComponents
	request.Limit = &limit
	request.Offset = &offset
	var response *essbasic.DescribeTemplatesResponse
	var callErr error
	subjectErr := c.withAgent(ctx, func(agent *essbasic.Agent) error {
		request.Agent = agent
		response, callErr = c.api.DescribeTemplatesWithContext(ctx, request)
		return callErr
	})
	if subjectErr != nil {
		return nil, providerCallError(subjectErr, callErr, false)
	}
	if response == nil || response.Response == nil || response.Response.RequestId == nil {
		return nil, &Error{Code: ErrorMalformedResponse}
	}
	return response, nil
}

// DescribeTemplatePreview returns one official short-lived H5 preview URL.
func (c *Client) DescribeTemplatePreview(ctx context.Context, templateID string) (string, error) {
	templateID = strings.TrimSpace(templateID)
	if !c.canCall(ctx, CapabilityTemplateRead) || !flowIDPattern.MatchString(templateID) {
		return "", &Error{Code: ErrorInvalidInput}
	}
	contentType := int64(1)
	withPreviewURL := true
	request := essbasic.NewDescribeTemplatesRequest()
	request.TemplateId = &templateID
	request.ContentType = &contentType
	request.WithPreviewUrl = &withPreviewURL
	var response *essbasic.DescribeTemplatesResponse
	var callErr error
	subjectErr := c.withAgent(ctx, func(agent *essbasic.Agent) error {
		request.Agent = agent
		response, callErr = c.api.DescribeTemplatesWithContext(ctx, request)
		return callErr
	})
	if subjectErr != nil {
		return "", providerCallError(subjectErr, callErr, false)
	}
	if response == nil || response.Response == nil || response.Response.RequestId == nil ||
		len(response.Response.Templates) != 1 || response.Response.Templates[0] == nil ||
		response.Response.Templates[0].TemplateId == nil ||
		strings.TrimSpace(*response.Response.Templates[0].TemplateId) != templateID ||
		response.Response.Templates[0].PreviewUrl == nil ||
		!validHTTPSURL(strings.TrimSpace(*response.Response.Templates[0].PreviewUrl)) {
		return "", &Error{Code: ErrorMalformedResponse}
	}
	return strings.TrimSpace(*response.Response.Templates[0].PreviewUrl), nil
}

// DescribeFlows queries at most one official batch of flow identifiers.
func (c *Client) DescribeFlows(
	ctx context.Context,
	flowIDs []string,
) (*essbasic.DescribeFlowDetailInfoResponse, error) {
	if !c.canCall(ctx, CapabilityFlowQuery) || len(flowIDs) < 1 || len(flowIDs) > 100 {
		return nil, &Error{Code: ErrorInvalidInput}
	}
	request := essbasic.NewDescribeFlowDetailInfoRequest()
	request.FlowIds = make([]*string, len(flowIDs))
	for index, flowID := range flowIDs {
		if !flowIDPattern.MatchString(flowID) {
			return nil, &Error{Code: ErrorInvalidInput}
		}
		value := flowID
		request.FlowIds[index] = &value
	}
	var response *essbasic.DescribeFlowDetailInfoResponse
	var callErr error
	subjectErr := c.withAgent(ctx, func(agent *essbasic.Agent) error {
		request.Agent = agent
		response, callErr = c.api.DescribeFlowDetailInfoWithContext(ctx, request)
		if callErr == nil && (response == nil || response.Response == nil ||
			stringValue(response.Response.ApplicationId) != stringValue(agent.AppId) ||
			stringValue(response.Response.ProxyOrganizationOpenId) !=
				stringValue(agent.ProxyOrganizationOpenId)) {
			callErr = &Error{Code: ErrorMalformedResponse}
		}
		return callErr
	})
	if subjectErr != nil {
		return nil, providerCallError(subjectErr, callErr, false)
	}
	if response == nil || response.Response == nil || response.Response.RequestId == nil {
		return nil, &Error{Code: ErrorMalformedResponse}
	}
	return response, nil
}

// CreateFlow creates exactly one flow through the database-authoritative write guard.
func (c *Client) CreateFlow(
	ctx context.Context,
	requestNo string,
	flow *essbasic.FlowInfo,
	preview bool,
) (CreateFlowResult, error) {
	if !c.canCall(ctx, CapabilityFlowCreate) || c.guard == nil || c.references == nil ||
		requestNo == "" || flow == nil {
		return CreateFlowResult{}, &Error{Code: ErrorInvalidInput}
	}
	if err := c.admitNewCharge(ctx); err != nil {
		return CreateFlowResult{}, err
	}
	operation := OperationFlowCreate
	if preview {
		operation = OperationFlowPreview
	}
	var result CreateFlowResult
	var operationErr error
	subjectErr := c.withAgent(ctx, func(agent *essbasic.Agent) error {
		releaseApprovers, bindErr := bindLocalMerchantApprovers(flow, agent)
		if bindErr != nil {
			operationErr = &Error{Code: ErrorInvalidInput, cause: bindErr}
			return operationErr
		}
		defer releaseApprovers()
		lease, err := c.guard.Acquire(ctx, supplierguard.Admission{
			RequestNo: requestNo, Provider: providerCode, OperationScope: operation,
		})
		if err != nil {
			operationErr = guardError(err)
			return operationErr
		}
		result = CreateFlowResult{RouteNo: lease.RouteNo, Recovered: lease.Recovered}
		if !lease.CallSupplier {
			operationErr = recoveredWriteError(lease)
			return operationErr
		}
		started := c.now()
		request := essbasic.NewCreateFlowsByTemplatesRequest()
		request.Agent = agent
		request.FlowInfos = []*essbasic.FlowInfo{flow}
		request.NeedPreview = &preview
		response, callErr := c.api.CreateFlowsByTemplatesWithContext(ctx, request)
		if callErr != nil {
			operationErr = c.finishFailedWrite(ctx, lease, started, safeError(callErr, true))
			return operationErr
		}
		result.RequestID, result.FlowID, result.PreviewURL, callErr = parseCreateFlowResponse(response, preview)
		if callErr != nil {
			var responseError *Error
			if !errors.As(callErr, &responseError) {
				responseError = &Error{Code: ErrorResultUnknown, cause: callErr}
			}
			operationErr = c.finishFailedWrite(ctx, lease, started, responseError)
			return operationErr
		}
		if !preview {
			_, bindErr := c.references.BindExternalReference(ctx, suppliercallback.ReferenceBinding{
				RouteNo: lease.RouteNo, Provider: providerCode, OperationScope: operation,
				ExternalReference: []byte(result.FlowID),
			})
			if bindErr != nil {
				operationErr = c.finishFailedWrite(
					ctx, lease, started, &Error{Code: ErrorResultUnknown, cause: bindErr},
				)
				return operationErr
			}
		}
		if observeErr := c.observeWrite(
			ctx, lease, supplierguard.ResultSucceeded, "tencent_ok", started,
		); observeErr != nil {
			operationErr = &Error{Code: ErrorResultUnknown, cause: observeErr}
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

// CreateReleaseFlow creates exactly one Tencent release agreement through the
// same database-authoritative write guard used for normal contract creation.
func (c *Client) CreateReleaseFlow(
	ctx context.Context,
	requestNo string,
	originalFlowID string,
	info *essbasic.RelieveInfo,
	deadlineUnix int64,
) (CreateFlowResult, error) {
	if !c.canCall(ctx, CapabilityFlowRelease) || c.guard == nil || c.references == nil ||
		requestNo == "" || !flowIDPattern.MatchString(originalFlowID) || info == nil ||
		deadlineUnix <= c.now().Unix() || deadlineUnix > c.now().Add(8*24*time.Hour).Unix() {
		return CreateFlowResult{}, &Error{Code: ErrorInvalidInput}
	}
	var result CreateFlowResult
	var operationErr error
	subjectErr := c.withAgent(ctx, func(agent *essbasic.Agent) error {
		lease, err := c.guard.Acquire(ctx, supplierguard.Admission{
			RequestNo: requestNo, Provider: providerCode, OperationScope: OperationFlowRelease,
		})
		if err != nil {
			operationErr = guardError(err)
			return operationErr
		}
		result = CreateFlowResult{RouteNo: lease.RouteNo, Recovered: lease.Recovered}
		if !lease.CallSupplier {
			operationErr = recoveredWriteError(lease)
			return operationErr
		}
		started := c.now()
		request := essbasic.NewChannelCreateReleaseFlowRequest()
		request.Agent = agent
		request.NeedRelievedFlowId = &originalFlowID
		request.ReliveInfo = info
		request.Deadline = &deadlineUnix
		response, callErr := c.api.ChannelCreateReleaseFlowWithContext(ctx, request)
		if callErr != nil {
			operationErr = c.finishFailedWrite(ctx, lease, started, safeError(callErr, true))
			return operationErr
		}
		result.RequestID, result.FlowID, callErr = parseReleaseFlowResponse(response)
		if callErr != nil {
			var responseError *Error
			if !errors.As(callErr, &responseError) {
				responseError = &Error{Code: ErrorResultUnknown, cause: callErr}
			}
			operationErr = c.finishFailedWrite(ctx, lease, started, responseError)
			return operationErr
		}
		if _, bindErr := c.references.BindExternalReference(ctx, suppliercallback.ReferenceBinding{
			RouteNo: lease.RouteNo, Provider: providerCode, OperationScope: OperationFlowRelease,
			ExternalReference: []byte(result.FlowID),
		}); bindErr != nil {
			operationErr = c.finishFailedWrite(
				ctx, lease, started, &Error{Code: ErrorResultUnknown, cause: bindErr},
			)
			return operationErr
		}
		if observeErr := c.observeWrite(ctx, lease, supplierguard.ResultSucceeded, "tencent_ok", started); observeErr != nil {
			operationErr = &Error{Code: ErrorResultUnknown, cause: observeErr}
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

func bindLocalMerchantApprovers(flow *essbasic.FlowInfo, agent *essbasic.Agent) (func(), error) {
	if flow == nil || agent == nil || agent.ProxyOperator == nil ||
		agent.ProxyOperator.OpenId == nil || agent.ProxyOrganizationOpenId == nil ||
		stringValue(agent.ProxyOperator.OpenId) == "" || stringValue(agent.ProxyOrganizationOpenId) == "" {
		return nil, errors.New("tencent local merchant subject is unavailable")
	}
	bound := make([]*essbasic.FlowApproverInfo, 0, len(flow.FlowApprovers))
	for _, approver := range flow.FlowApprovers {
		if approver == nil || stringValue(approver.ApproverType) != "ORGANIZATION" ||
			approver.NotChannelOrganization == nil || *approver.NotChannelOrganization ||
			approver.Name != nil || approver.Mobile != nil || approver.OpenId != nil ||
			approver.OrganizationOpenId != nil {
			continue
		}
		openID := stringValue(agent.ProxyOperator.OpenId)
		organizationOpenID := stringValue(agent.ProxyOrganizationOpenId)
		approver.OpenId = &openID
		approver.OrganizationOpenId = &organizationOpenID
		bound = append(bound, approver)
	}
	if len(bound) == 0 {
		return func() {}, nil
	}
	return func() {
		for _, approver := range bound {
			approver.OpenId = nil
			approver.OrganizationOpenId = nil
		}
	}, nil
}

// CreateSignURL creates an on-demand 30-minute H5 link and never accepts credentials or endpoints.
func (c *Client) CreateSignURL(
	ctx context.Context,
	flowID string,
	approvers []*essbasic.FlowApproverInfo,
	jumpURL string,
) (*essbasic.ChannelCreateFlowSignUrlResponse, error) {
	if !c.canCall(ctx, CapabilitySignURL) || !flowIDPattern.MatchString(flowID) ||
		!validApprovers(approvers) || !validHTTPSURL(jumpURL) {
		return nil, &Error{Code: ErrorInvalidInput}
	}
	urlType := int64(0)
	request := essbasic.NewChannelCreateFlowSignUrlRequest()
	request.FlowId = &flowID
	request.FlowApproverInfos = approvers
	request.JumpUrl = &jumpURL
	request.UrlType = &urlType
	request.ExpiredOn = common.Int64Ptr(signURLLifetimeSeconds)
	var response *essbasic.ChannelCreateFlowSignUrlResponse
	var callErr error
	subjectErr := c.withAgent(ctx, func(agent *essbasic.Agent) error {
		request.Agent = agent
		response, callErr = c.api.ChannelCreateFlowSignUrlWithContext(ctx, request)
		return callErr
	})
	if subjectErr != nil {
		return nil, providerCallError(subjectErr, callErr, false)
	}
	if response == nil || response.Response == nil || response.Response.RequestId == nil ||
		!validSignURLResponse(response.Response.FlowApproverUrlInfos) {
		return nil, &Error{Code: ErrorMalformedResponse}
	}
	return response, nil
}

// GetDownloadURL returns Tencent's one-use five-minute download-console URL.
func (c *Client) GetDownloadURL(
	ctx context.Context,
	flows []*essbasic.DownloadFlowInfo,
) (*essbasic.GetDownloadFlowUrlResponse, error) {
	if !c.canCall(ctx, CapabilityDownload) || !validDownloadFlows(flows) {
		return nil, &Error{Code: ErrorInvalidInput}
	}
	request := essbasic.NewGetDownloadFlowUrlRequest()
	request.DownLoadFlows = flows
	var response *essbasic.GetDownloadFlowUrlResponse
	var callErr error
	subjectErr := c.withAgent(ctx, func(agent *essbasic.Agent) error {
		request.Agent = agent
		response, callErr = c.api.GetDownloadFlowUrlWithContext(ctx, request)
		return callErr
	})
	if subjectErr != nil {
		return nil, providerCallError(subjectErr, callErr, false)
	}
	if response == nil || response.Response == nil || response.Response.RequestId == nil ||
		response.Response.DownLoadUrl == nil || !validHTTPSURL(*response.Response.DownLoadUrl) {
		return nil, &Error{Code: ErrorMalformedResponse}
	}
	return response, nil
}

// Query implements the installation-local read-only reconciliation contract.
func (c *Client) Query(
	ctx context.Context,
	input suppliercallback.QueryRequest,
) (suppliercallback.QueryResponse, error) {
	flowID := string(input.ExternalReference)
	if !flowIDPattern.MatchString(flowID) {
		return suppliercallback.QueryResponse{
			FailureCode: suppliercallback.QueryFailureReferenceMismatch,
		}, &Error{Code: ErrorInvalidInput}
	}
	response, err := c.DescribeFlows(ctx, []string{flowID})
	if err != nil {
		return suppliercallback.QueryResponse{FailureCode: queryFailureCode(err)}, err
	}
	flow := findFlow(response.Response.FlowInfo, flowID)
	if flow == nil {
		return suppliercallback.QueryResponse{
			FailureCode: suppliercallback.QueryFailureReferenceMismatch,
		}, &Error{Code: ErrorResourceNotFound}
	}
	status, valid := normalizedFlowStatus(flow.FlowStatus)
	if !valid {
		return suppliercallback.QueryResponse{
			FailureCode: suppliercallback.QueryFailureMalformedResponse,
		}, &Error{Code: ErrorMalformedResponse}
	}
	evidence, marshalErr := json.Marshal(response)
	if marshalErr != nil || len(evidence) == 0 || len(evidence) > maximumEvidenceBytes {
		clearBytes(evidence)
		return suppliercallback.QueryResponse{
			FailureCode: suppliercallback.QueryFailureMalformedResponse,
		}, &Error{Code: ErrorMalformedResponse, cause: marshalErr}
	}
	return suppliercallback.QueryResponse{
		Status: status, ProviderStatus: stringValue(flow.FlowStatus),
		EvidenceFrame: evidence, SupplierObservedAt: c.now().UTC(),
	}, nil
}

// CreateFlowResult is the safe local outcome of one guarded official request.
type CreateFlowResult struct {
	// RouteNo identifies the durable local supplier-write route.
	RouteNo string
	// FlowID is the Tencent flow identifier returned by a new successful write.
	FlowID string
	// PreviewURL is the optional short-lived Tencent preview URL.
	PreviewURL string
	// RequestID is the Tencent request identifier for support correlation.
	RequestID string
	// Recovered reports that the local write guard returned an existing attempt.
	Recovered bool
}

func (c *Client) canCall(ctx context.Context, capability Capability) bool {
	return c.Configured() && c.HasCapability(capability) && ctx != nil && ctx.Err() == nil
}

func (c *Client) admitNewCharge(ctx context.Context) error {
	if c == nil || c.chargeGate == nil {
		return nil
	}
	if err := c.chargeGate.AdmitNewCharge(ctx, providerCode); err != nil {
		return &Error{Code: ErrorPermissionDenied, cause: err}
	}
	return nil
}

func (c *Client) finishFailedWrite(
	ctx context.Context,
	lease supplierguard.Lease,
	started time.Time,
	failure *Error,
) error {
	if failure == nil {
		failure = &Error{Code: ErrorResultUnknown}
	}
	if err := c.observeWrite(ctx, lease, writeResultClass(failure), string(failure.Code), started); err != nil {
		return &Error{Code: ErrorResultUnknown, cause: err}
	}
	return failure
}

func (c *Client) observeWrite(
	ctx context.Context,
	lease supplierguard.Lease,
	result supplierguard.ResultClass,
	code string,
	started time.Time,
) error {
	elapsed := c.now().Sub(started).Round(time.Millisecond)
	if elapsed < 0 {
		elapsed = 0
	}
	if elapsed > time.Hour {
		elapsed = time.Hour
	}
	_, err := c.guard.Observe(ctx, supplierguard.Observation{
		AttemptNo: lease.AttemptNo, Result: result, StableCode: code, Elapsed: elapsed,
	})
	return err
}

func parseCreateFlowResponse(
	response *essbasic.CreateFlowsByTemplatesResponse,
	preview bool,
) (requestID, flowID, previewURL string, err error) {
	if response == nil || response.Response == nil || response.Response.RequestId == nil ||
		!validOpaque(stringValue(response.Response.RequestId), 1, 128) {
		return "", "", "", &Error{Code: ErrorMalformedResponse}
	}
	requestID = stringValue(response.Response.RequestId)
	if len(response.Response.ErrorMessages) > 0 &&
		strings.TrimSpace(stringValue(response.Response.ErrorMessages[0])) != "" {
		return "", "", "", &Error{Code: ErrorRejected, RequestID: requestID}
	}
	if preview {
		if len(response.Response.PreviewUrls) != 1 ||
			!validHTTPSURL(stringValue(response.Response.PreviewUrls[0])) {
			return "", "", "", &Error{Code: ErrorMalformedResponse, RequestID: requestID}
		}
		return requestID, "", stringValue(response.Response.PreviewUrls[0]), nil
	}
	if len(response.Response.FlowIds) != 1 || !flowIDPattern.MatchString(stringValue(response.Response.FlowIds[0])) {
		return "", "", "", &Error{Code: ErrorResultUnknown, RequestID: requestID}
	}
	return requestID, stringValue(response.Response.FlowIds[0]), "", nil
}

func parseReleaseFlowResponse(
	response *essbasic.ChannelCreateReleaseFlowResponse,
) (requestID, flowID string, err error) {
	if response == nil || response.Response == nil || response.Response.RequestId == nil ||
		!validOpaque(stringValue(response.Response.RequestId), 1, 128) {
		return "", "", &Error{Code: ErrorMalformedResponse}
	}
	requestID = stringValue(response.Response.RequestId)
	if response.Response.FlowId == nil || !flowIDPattern.MatchString(stringValue(response.Response.FlowId)) {
		return "", "", &Error{Code: ErrorResultUnknown, RequestID: requestID}
	}
	return requestID, stringValue(response.Response.FlowId), nil
}

func guardError(err error) error {
	switch {
	case errors.Is(err, supplierguard.ErrRateLimited):
		return &Error{Code: ErrorRateLimited, cause: err}
	case errors.Is(err, supplierguard.ErrCircuitOpen):
		return &Error{Code: ErrorServiceUnavailable, cause: err}
	case errors.Is(err, supplierguard.ErrInvalid), errors.Is(err, supplierguard.ErrConflict):
		return &Error{Code: ErrorInvalidInput, cause: err}
	default:
		return &Error{Code: ErrorServiceUnavailable, cause: err}
	}
}

func recoveredWriteError(lease supplierguard.Lease) error {
	switch lease.Result {
	case supplierguard.ResultSucceeded:
		return nil
	case supplierguard.ResultAvailabilityFailed:
		return &Error{Code: ErrorServiceUnavailable}
	case supplierguard.ResultDefinitiveFailure:
		return &Error{Code: ErrorRejected}
	case supplierguard.ResultUnknown:
		return &Error{Code: ErrorResultUnknown}
	default:
		return &Error{Code: ErrorResultUnknown}
	}
}

func queryFailureCode(err error) suppliercallback.QueryFailureCode {
	var safe *Error
	if !errors.As(err, &safe) {
		return suppliercallback.QueryFailureTransportUnavailable
	}
	switch safe.Code {
	case ErrorTimeout:
		return suppliercallback.QueryFailureTimeout
	case ErrorMalformedResponse:
		return suppliercallback.QueryFailureMalformedResponse
	case ErrorResourceNotFound:
		return suppliercallback.QueryFailureReferenceMismatch
	case ErrorCredentialRejected, ErrorPermissionDenied, ErrorSubjectUnavailable,
		ErrorInvalidInput, ErrorRejected:
		return suppliercallback.QueryFailureProviderRejected
	default:
		return suppliercallback.QueryFailureTransportUnavailable
	}
}

func providerCallError(subjectErr, callErr error, write bool) error {
	if callErr != nil {
		var alreadySafe *Error
		if errors.As(callErr, &alreadySafe) {
			return alreadySafe
		}
		return safeError(callErr, write)
	}
	return &Error{Code: ErrorSubjectUnavailable, cause: subjectErr}
}

func findFlow(flows []*essbasic.FlowDetailInfo, flowID string) *essbasic.FlowDetailInfo {
	for _, flow := range flows {
		if flow != nil && stringValue(flow.FlowId) == flowID {
			return flow
		}
	}
	return nil
}

func normalizedFlowStatus(value *string) (suppliercallback.Status, bool) {
	switch stringValue(value) {
	case "INIT", "PART", "WILLEXPIRE":
		return suppliercallback.StatusRunning, true
	case "ALL", "RELIEVED":
		return suppliercallback.StatusSucceeded, true
	case "REJECT", "DEADLINE", "CANCEL", "INVALID", "EXCEPTION":
		return suppliercallback.StatusFailed, true
	default:
		return suppliercallback.StatusResultUnknown, false
	}
}

func validHTTPSURL(value string) bool {
	if len(value) < 1 || len(value) > 1000 || strings.ContainsAny(value, "\r\n\t\x00") {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil &&
		parsed.Fragment == ""
}

func validApprovers(values []*essbasic.FlowApproverInfo) bool {
	if len(values) < 1 || len(values) > 20 {
		return false
	}
	for _, value := range values {
		if value == nil {
			return false
		}
	}
	return true
}

func validSignURLResponse(values []*essbasic.FlowApproverUrlInfo) bool {
	if len(values) < 1 || len(values) > 20 {
		return false
	}
	for _, value := range values {
		if value == nil || (!validHTTPSURL(stringValue(value.SignUrl)) &&
			!validHTTPSURL(stringValue(value.LongUrl))) {
			return false
		}
	}
	return true
}

func validDownloadFlows(values []*essbasic.DownloadFlowInfo) bool {
	if len(values) < 1 || len(values) > 50 {
		return false
	}
	total := 0
	for _, value := range values {
		if value == nil || !validOpaque(stringValue(value.FileName), 1, 200) ||
			len(value.FlowIdList) < 1 || len(value.FlowIdList) > 20 {
			return false
		}
		total += len(value.FlowIdList)
		if total > 50 {
			return false
		}
		for _, flowID := range value.FlowIdList {
			if !flowIDPattern.MatchString(stringValue(flowID)) {
				return false
			}
		}
	}
	return true
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
