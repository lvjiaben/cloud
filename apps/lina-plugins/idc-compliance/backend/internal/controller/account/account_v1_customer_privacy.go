// Customer privacy controllers derive ownership from authentication and map safe results. They never accept a customer number from browser input.

package account

import (
	"context"
	"time"

	"lina-core/pkg/bizerr"
	v1 "lina-plugin-idc-compliance/backend/api/account/v1"
	"lina-plugin-idc-compliance/backend/internal/service/privacygovernance"
	"lina-plugin-idc-core/backend/cap/identitycap"
)

// CustomerPrivacyGet returns published notices and only the current customer's records.
func (c *ControllerV1) CustomerPrivacyGet(
	ctx context.Context,
	_ *v1.CustomerPrivacyGetReq,
) (*v1.CustomerPrivacyGetRes, error) {
	overview, err := c.privacy.CurrentCustomerPrivacy(ctx)
	if err != nil {
		return nil, err
	}
	response := &v1.CustomerPrivacyGetRes{
		Notices:  make([]v1.CustomerPrivacyNotice, 0, len(overview.Notices)),
		Consents: make([]v1.CustomerPrivacyConsent, 0, len(overview.Consents)),
		Requests: make([]v1.CustomerPrivacyRequest, 0, len(overview.Requests)),
	}
	for _, notice := range overview.Notices {
		response.Notices = append(response.Notices, v1.CustomerPrivacyNotice{
			AssessmentTypeCode: string(notice.AssessmentType), AssessmentNo: notice.AssessmentNo,
			Version: notice.Version, Purpose: notice.Purpose, Necessity: notice.Necessity,
			Impact: notice.Impact, Recipients: notice.Recipients, StorageLocation: notice.StorageLocation,
			RetentionDescription: notice.RetentionDescription, RetentionDays: notice.RetentionDays,
			RightsEntry: notice.RightsEntry,
		})
	}
	for _, consent := range overview.Consents {
		response.Consents = append(response.Consents, projectCustomerPrivacyConsent(consent))
	}
	for _, request := range overview.Requests {
		response.Requests = append(response.Requests, projectCustomerPrivacyRequest(request))
	}
	return response, nil
}

// CustomerPrivacyConsentWithdrawCreate records a future-processing withdrawal.
func (c *ControllerV1) CustomerPrivacyConsentWithdrawCreate(
	ctx context.Context,
	req *v1.CustomerPrivacyConsentWithdrawCreateReq,
) (*v1.CustomerPrivacyConsentWithdrawCreateRes, error) {
	if req == nil {
		return nil, bizerr.NewCode(privacygovernance.CodeInvalid)
	}
	decision, err := c.privacy.WithdrawCurrentCustomerConsent(ctx, identitycap.PrivacyConsentWithdrawalInput{
		ConsentType:    identitycap.PrivacyConsentType(req.ConsentTypeCode),
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return &v1.CustomerPrivacyConsentWithdrawCreateRes{Consent: projectCustomerPrivacyConsent(*decision)}, nil
}

// CustomerPrivacyRequestCreate submits or safely replays one customer request.
func (c *ControllerV1) CustomerPrivacyRequestCreate(
	ctx context.Context,
	req *v1.CustomerPrivacyRequestCreateReq,
) (*v1.CustomerPrivacyRequestCreateRes, error) {
	if req == nil {
		return nil, bizerr.NewCode(privacygovernance.CodeInvalid)
	}
	request, err := c.privacy.CreateCurrentCustomerRequest(ctx, privacygovernance.CustomerPrivacyRequestInput{
		RequestType: req.RequestTypeCode, IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return &v1.CustomerPrivacyRequestCreateRes{Request: projectCustomerPrivacyRequest(*request)}, nil
}

func projectCustomerPrivacyConsent(consent identitycap.PrivacyConsentDecision) v1.CustomerPrivacyConsent {
	return v1.CustomerPrivacyConsent{
		ConsentNo: consent.ConsentNo, ConsentTypeCode: string(consent.ConsentType),
		ActionCode: consent.ActionCode, DecidedAt: consent.OccurredAt.UTC().UnixMilli(),
	}
}

func projectCustomerPrivacyRequest(request privacygovernance.CustomerRequest) v1.CustomerPrivacyRequest {
	return v1.CustomerPrivacyRequest{
		RequestNo: request.RequestNo, RequestTypeCode: request.RequestType,
		StatusCode: request.StatusCode, HoldReasonCode: request.HoldReasonCode,
		SubmittedAt: request.SubmittedAt.UTC().UnixMilli(), CompletedAt: optionalUnixMillis(request.CompletedAt),
		Version: request.Version,
	}
}

func optionalUnixMillis(value *time.Time) int64 {
	if value == nil || value.IsZero() {
		return 0
	}
	return value.UTC().UnixMilli()
}
