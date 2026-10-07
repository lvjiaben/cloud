// This file probes buyer-account capabilities through bounded read-only calls
// and projects only stable, credential-free availability states.

package tencentess

import (
	"context"
	"errors"
	"time"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	essbasic "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/essbasic/v20210526"
)

const (
	capabilityCheckTimeout         = 10 * time.Second
	maximumExtendedServiceStatuses = 64
	extendedServiceAutoSign        = "AUTO_SIGN"
	extendedServiceDownload        = "DOWNLOAD_FLOW"
	extendedServiceEnabled         = "ENABLE"
	extendedServiceDisabled        = "DISABLE"
)

// CapabilityState is the safe management projection for one runtime ability.
type CapabilityState string

const (
	// CapabilityDisabled means the installation did not enable this ability.
	CapabilityDisabled CapabilityState = "disabled"
	// CapabilityAvailable means a bounded read-only probe confirmed this ability.
	CapabilityAvailable CapabilityState = "available"
	// CapabilityUnavailable means a bounded check rejected this ability.
	CapabilityUnavailable CapabilityState = "unavailable"
	// CapabilityOperationCheckRequired defers confirmation to a real operation.
	CapabilityOperationCheckRequired CapabilityState = "operation_check_required"
)

// CapabilityStatus contains no credential, subject identifier, or provider free text.
type CapabilityStatus struct {
	// Capability identifies the independently checked ability.
	Capability Capability
	// State is the safe closed availability projection.
	State CapabilityState
	// Code explains an unavailable state without provider free text.
	Code ErrorCode
	// CheckedAt records when this local process made the decision.
	CheckedAt time.Time
}

// CheckCapabilities performs only bounded read-only probes. Abilities that require
// a real flow or a write remain explicitly marked for per-operation verification.
func (c *Client) CheckCapabilities(ctx context.Context) []CapabilityStatus {
	capabilities := []Capability{
		CapabilityTemplateRead,
		CapabilityFlowCreate,
		CapabilityFlowRelease,
		CapabilitySignURL,
		CapabilityFlowQuery,
		CapabilityDownload,
		CapabilityEvidenceReport,
		CapabilityAutoSign,
		CapabilityUsageRead,
		CapabilityAccountBill,
	}
	result := make([]CapabilityStatus, 0, len(capabilities))
	if c == nil || !c.Configured() || ctx == nil || ctx.Err() != nil {
		for _, capability := range capabilities {
			result = append(result, CapabilityStatus{
				Capability: capability, State: CapabilityUnavailable,
				Code: ErrorNotConfigured,
			})
		}
		return result
	}
	subjectErr := c.withAgent(ctx, func(*essbasic.Agent) error { return nil })
	var (
		extendedServices map[string]string
		extendedError    error
	)
	if subjectErr == nil && (c.HasCapability(CapabilityAutoSign) || c.HasCapability(CapabilityDownload)) {
		extendedServices, extendedError = c.checkExtendedServices(ctx)
	}
	for _, capability := range capabilities {
		status := CapabilityStatus{
			Capability: capability, State: CapabilityDisabled, CheckedAt: c.now().UTC(),
		}
		if !c.HasCapability(capability) {
			result = append(result, status)
			continue
		}
		if subjectErr != nil {
			status.State = CapabilityUnavailable
			status.Code = ErrorSubjectUnavailable
			result = append(result, status)
			continue
		}
		switch capability {
		case CapabilityTemplateRead:
			status = c.checkTemplates(ctx, status)
		case CapabilityAutoSign:
			status = extendedCapability(status, extendedServices, extendedError, extendedServiceAutoSign)
		case CapabilityDownload:
			status = extendedCapability(status, extendedServices, extendedError, extendedServiceDownload)
		case CapabilityUsageRead:
			status = c.checkUsage(ctx, status)
		case CapabilityAccountBill:
			status = c.checkAccountBill(ctx, status)
		default:
			status.State = CapabilityOperationCheckRequired
		}
		result = append(result, status)
	}
	return result
}

// CheckSealAndDownload performs the single bounded extended-service probe used
// by the final onboarding step. Disabled deployment abilities remain disabled.
func (c *Client) CheckSealAndDownload(ctx context.Context) []CapabilityStatus {
	capabilities := []Capability{CapabilityAutoSign, CapabilityDownload}
	result := make([]CapabilityStatus, 0, len(capabilities))
	if c == nil || !c.Configured() || ctx == nil || ctx.Err() != nil {
		for _, capability := range capabilities {
			result = append(result, CapabilityStatus{
				Capability: capability, State: CapabilityUnavailable, Code: ErrorNotConfigured,
			})
		}
		return result
	}
	if err := c.withAgent(ctx, func(*essbasic.Agent) error { return nil }); err != nil {
		for _, capability := range capabilities {
			result = append(result, CapabilityStatus{
				Capability: capability, State: CapabilityUnavailable, Code: ErrorSubjectUnavailable,
				CheckedAt: c.now().UTC(),
			})
		}
		return result
	}
	services, checkErr := c.checkExtendedServices(ctx)
	for _, capability := range capabilities {
		status := CapabilityStatus{
			Capability: capability, State: CapabilityDisabled, CheckedAt: c.now().UTC(),
		}
		if !c.HasCapability(capability) {
			result = append(result, status)
			continue
		}
		serviceType := extendedServiceAutoSign
		if capability == CapabilityDownload {
			serviceType = extendedServiceDownload
		}
		result = append(result, extendedCapability(status, services, checkErr, serviceType))
	}
	return result
}

func (c *Client) checkTemplates(ctx context.Context, status CapabilityStatus) CapabilityStatus {
	probeContext, cancel := context.WithTimeout(ctx, capabilityCheckTimeout)
	defer cancel()
	if _, err := c.DescribeTemplates(probeContext, 1, 0); err != nil {
		return failedCapability(status, err)
	}
	status.State = CapabilityAvailable
	return status
}

func (c *Client) checkExtendedServices(ctx context.Context) (map[string]string, error) {
	probeContext, cancel := context.WithTimeout(ctx, capabilityCheckTimeout)
	defer cancel()
	request := essbasic.NewDescribeExtendedServiceAuthInfoRequest()
	var response *essbasic.DescribeExtendedServiceAuthInfoResponse
	var callErr error
	subjectErr := c.withAgent(probeContext, func(agent *essbasic.Agent) error {
		request.Agent = agent
		response, callErr = c.api.DescribeExtendedServiceAuthInfoWithContext(probeContext, request)
		return callErr
	})
	if subjectErr != nil {
		return nil, providerCallError(subjectErr, callErr, false)
	}
	if response == nil || response.Response == nil || response.Response.RequestId == nil ||
		len(response.Response.AuthInfo) > maximumExtendedServiceStatuses {
		return nil, &Error{Code: ErrorMalformedResponse}
	}
	result := make(map[string]string, len(response.Response.AuthInfo))
	for _, item := range response.Response.AuthInfo {
		if item == nil {
			return nil, &Error{Code: ErrorMalformedResponse}
		}
		serviceType := stringValue(item.Type)
		serviceStatus := stringValue(item.Status)
		if !validOpaque(serviceType, 1, 64) ||
			(serviceStatus != extendedServiceEnabled && serviceStatus != extendedServiceDisabled) {
			return nil, &Error{Code: ErrorMalformedResponse}
		}
		if _, duplicate := result[serviceType]; duplicate {
			return nil, &Error{Code: ErrorMalformedResponse}
		}
		result[serviceType] = serviceStatus
	}
	return result, nil
}

func (c *Client) checkUsage(ctx context.Context, status CapabilityStatus) CapabilityStatus {
	probeContext, cancel := context.WithTimeout(ctx, capabilityCheckTimeout)
	defer cancel()
	day := c.now().UTC().Format("20060102")
	request := essbasic.NewChannelDescribeBillUsageDetailRequest()
	request.StartTime = &day
	request.EndTime = &day
	request.Offset = common.Int64Ptr(0)
	request.Limit = common.Int64Ptr(1)
	var response *essbasic.ChannelDescribeBillUsageDetailResponse
	var callErr error
	subjectErr := c.withAgent(probeContext, func(agent *essbasic.Agent) error {
		request.Agent = agent
		response, callErr = c.api.ChannelDescribeBillUsageDetailWithContext(probeContext, request)
		return callErr
	})
	if subjectErr != nil {
		return failedCapability(status, providerCallError(subjectErr, callErr, false))
	}
	if response == nil || response.Response == nil || response.Response.RequestId == nil ||
		response.Response.Total == nil {
		return failedCapability(status, &Error{Code: ErrorMalformedResponse})
	}
	status.State = CapabilityAvailable
	return status
}

func (c *Client) checkAccountBill(ctx context.Context, status CapabilityStatus) CapabilityStatus {
	probeContext, cancel := context.WithTimeout(ctx, capabilityCheckTimeout)
	defer cancel()
	request := essbasic.NewChannelDescribeAccountBillDetailRequest()
	var response *essbasic.ChannelDescribeAccountBillDetailResponse
	var callErr error
	subjectErr := c.withAgent(probeContext, func(agent *essbasic.Agent) error {
		request.Agent = agent
		response, callErr = c.api.ChannelDescribeAccountBillDetailWithContext(probeContext, request)
		return callErr
	})
	if subjectErr != nil {
		return failedCapability(status, providerCallError(subjectErr, callErr, false))
	}
	if response == nil || response.Response == nil || response.Response.RequestId == nil ||
		response.Response.BoundAccountsNumber == nil ||
		response.Response.RemainAvailableAccountsNumber == nil {
		return failedCapability(status, &Error{Code: ErrorMalformedResponse})
	}
	status.State = CapabilityAvailable
	return status
}

func failedCapability(status CapabilityStatus, err error) CapabilityStatus {
	status.State = CapabilityUnavailable
	var safe *Error
	if errors.As(err, &safe) {
		status.Code = safe.Code
	} else {
		status.Code = ErrorServiceUnavailable
	}
	return status
}

func extendedCapability(
	status CapabilityStatus,
	services map[string]string,
	err error,
	serviceType string,
) CapabilityStatus {
	if err != nil {
		return failedCapability(status, err)
	}
	if services[serviceType] != extendedServiceEnabled {
		status.State = CapabilityUnavailable
		status.Code = ErrorPermissionDenied
		return status
	}
	status.State = CapabilityAvailable
	return status
}
