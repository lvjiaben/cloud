package notification

import (
	"context"
	"crypto/hmac"
	"database/sql"
	"encoding/json"
	"errors"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/supportcap"
	modeldo "lina-plugin-idc-notify/backend/internal/model/do"
	"lina-plugin-idc-notify/backend/internal/model/entity"
)

const presalesAccessLinkEvent = "support.presales_access_link"

func (s *serviceImpl) CreateAnonymousPresalesTicket(ctx context.Context, input PresalesTicketInput) (*PresalesAccess, error) {
	defer clear(input.BrowserNonce)
	defer clear(input.Destination)
	defer clear(input.Code)
	if !validateIdempotencyKey(input.OperationNo) || input.VerificationNo == "" || len(input.VerificationNo) > 48 ||
		input.CategoryNo == "" || input.Subject == "" || input.Body == "" {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	binding, err := s.preparePresalesBinding(ctx, PresalesBindingInput{BrowserNonce: input.BrowserNonce,
		Destination: input.Destination, DestinationKind: input.DestinationKind, Locale: input.Locale, Operation: "create"})
	if err != nil {
		return nil, err
	}
	defer clear(binding.destination)
	operationDigest, err := supportcap.OperationDigest(input.OperationNo)
	if err != nil {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	supportRequest := supportcap.CreateAnonymousPresalesRequest{
		DestinationKind: supportcap.DestinationKind(binding.destinationKind), DestinationDigest: binding.destinationDigest,
		CategoryNo: input.CategoryNo, Subject: input.Subject, Body: input.Body,
		CustomFields: input.CustomFields, OperationNo: input.OperationNo,
	}
	if supportcap.ValidateCreateRequest(supportRequest) != nil {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	customFieldsJSON, err := json.Marshal(canonicalPresalesCustomFields(input.CustomFields))
	if err != nil {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	defer clear(customFieldsJSON)
	requestDigest := digestText("create", binding.destinationKind, binding.destinationDigest, binding.browserDigest,
		input.VerificationNo, input.CategoryNo, input.Subject, input.Body, string(customFieldsJSON))
	operation, err := s.preparePresalesOperation(ctx, operationDigest, requestDigest, input.VerificationNo, input.Code, binding)
	if err != nil {
		return nil, err
	}
	result, err := s.support.CreateAnonymousPresales(ctx, supportRequest)
	if err != nil || result == nil || supportcap.ValidateCreateResult(*result) != nil {
		return nil, errorOrCode(err, CodeUnavailable)
	}
	return s.completePresalesOperation(ctx, operation, binding, result.TicketNo, result.AccessToken, result.ExpiresAt)
}

func canonicalPresalesCustomFields(items []supportcap.CustomFieldValue) []supportcap.CustomFieldValue {
	result := make([]supportcap.CustomFieldValue, len(items))
	copy(result, items)
	for index := range result {
		result[index].OptionNos = append([]string(nil), result[index].OptionNos...)
		sort.Strings(result[index].OptionNos)
	}
	sort.Slice(result, func(left, right int) bool { return result[left].FieldNo < result[right].FieldNo })
	return result
}

func (s *serviceImpl) RenewAnonymousPresalesAccess(ctx context.Context, input PresalesRenewalInput) (*PresalesAccess, error) {
	defer clear(input.BrowserNonce)
	defer clear(input.Destination)
	defer clear(input.Code)
	if !validateIdempotencyKey(input.OperationNo) || input.VerificationNo == "" || len(input.VerificationNo) > 48 ||
		!presalesTicketPattern.MatchString(input.TicketNo) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	binding, err := s.preparePresalesBinding(ctx, PresalesBindingInput{BrowserNonce: input.BrowserNonce,
		Destination: input.Destination, DestinationKind: input.DestinationKind, Locale: input.Locale,
		Operation: "renew", TicketNo: input.TicketNo})
	if err != nil {
		return nil, err
	}
	defer clear(binding.destination)
	operationDigest, err := supportcap.RenewOperationDigest(input.OperationNo)
	if err != nil {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	requestDigest := digestText("renew", binding.destinationKind, binding.destinationDigest, binding.browserDigest,
		binding.ticketDigest, input.VerificationNo)
	supportRequest := supportcap.RenewAnonymousPresalesAccessRequest{
		DestinationKind: supportcap.DestinationKind(binding.destinationKind), DestinationDigest: binding.destinationDigest,
		TicketNo: input.TicketNo, OperationNo: input.OperationNo,
	}
	if supportcap.ValidateRenewRequest(supportRequest) != nil {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	operation, err := s.preparePresalesOperation(ctx, operationDigest, requestDigest, input.VerificationNo, input.Code, binding)
	if err != nil {
		return nil, err
	}
	result, err := s.support.RenewAnonymousPresalesAccess(ctx, supportRequest)
	if err != nil || result == nil || supportcap.ValidateRenewResult(*result) != nil {
		return nil, errorOrCode(err, CodeUnavailable)
	}
	return s.completePresalesOperation(ctx, operation, binding, result.TicketNo, result.AccessToken, result.ExpiresAt)
}

func (s *serviceImpl) preparePresalesOperation(ctx context.Context, operationDigest, requestDigest, verificationNo string,
	code []byte, binding *presalesBinding) (*entity.PresalesOperation, error) {
	if binding == nil || len(code) != 6 {
		return nil, bizerr.NewCode(CodeVerificationInvalid)
	}
	for _, digit := range code {
		if digit < '0' || digit > '9' {
			return nil, bizerr.NewCode(CodeVerificationInvalid)
		}
	}
	if existing, err := s.loadPresalesOperation(ctx, operationDigest); err != nil {
		return nil, publicError(err)
	} else if existing != nil {
		if existing.RequestDigest != requestDigest {
			return nil, bizerr.NewCode(CodeConflict)
		}
		return existing, nil
	}
	codeValue := append([]byte(verificationNo+":"), code...)
	digests, err := s.hmacDigests(ctx, map[string][]byte{"code": codeValue})
	clear(codeValue)
	if err != nil {
		return nil, err
	}
	prepared, publicErr, err := s.insertPresalesOperation(ctx, operationDigest, requestDigest, verificationNo, digests["code"], binding)
	if err != nil {
		if existing, loadErr := s.loadPresalesOperation(ctx, operationDigest); loadErr == nil && existing != nil {
			if existing.RequestDigest != requestDigest {
				return nil, bizerr.NewCode(CodeConflict)
			}
			return existing, nil
		}
		return nil, publicError(err)
	}
	if publicErr != nil {
		return nil, publicErr
	}
	if prepared == nil || prepared.RequestDigest != requestDigest {
		return nil, bizerr.NewCode(CodeConflict)
	}
	return prepared, nil
}

func (s *serviceImpl) insertPresalesOperation(ctx context.Context, operationDigest, requestDigest,
	verificationNo, codeDigest string, binding *presalesBinding) (*entity.PresalesOperation, error, error) {
	var prepared *entity.PresalesOperation
	var publicErr error
	err := s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		if existing, loadErr := s.loadPresalesOperation(txCtx, operationDigest); loadErr != nil {
			return loadErr
		} else if existing != nil {
			prepared = existing
			return nil
		}
		businessErr, verifyErr := s.consumePresalesVerification(txCtx, verificationNo, codeDigest, binding)
		if verifyErr != nil {
			return verifyErr
		}
		if businessErr != nil {
			publicErr = businessErr
			return nil
		}
		_, insertErr := s.database.Model(presalesOperationTable).Ctx(txCtx).Data(modeldo.PresalesOperation{
			TenantId: notificationTenantID, OperationDigest: operationDigest, OperationCode: binding.operation,
			VerificationNo: verificationNo, DestinationKind: binding.destinationKind, DestinationDigest: binding.destinationDigest,
			BrowserDigest: binding.browserDigest, TicketDigest: binding.ticketDigest, RequestDigest: requestDigest,
			StatusCode: "prepared", LinkDeliveryState: "not_started", ProviderBoundaryCrossed: false,
		}).Insert()
		if insertErr != nil {
			return insertErr
		}
		prepared, insertErr = s.loadPresalesOperation(txCtx, operationDigest)
		return insertErr
	})
	return prepared, publicErr, err
}

func (s *serviceImpl) consumePresalesVerification(ctx context.Context, verificationNo, codeDigest string,
	binding *presalesBinding) (error, error) {
	var verification entity.ContactVerification
	if err := s.database.Model(contactVerificationTable).Ctx(ctx).
		Where(modeldo.ContactVerification{TenantId: notificationTenantID, VerificationNo: verificationNo}).
		LockUpdate().Scan(&verification); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return bizerr.NewCode(CodeVerificationInvalid), nil
		}
		return nil, err
	}
	if !verificationMatchesBinding(verification, binding) || verificationIsTerminal(verification.StatusCode) {
		return bizerr.NewCode(CodeVerificationInvalid), nil
	}
	nowValue, err := s.database.GetValue(ctx, "SELECT CURRENT_TIMESTAMP")
	if err != nil {
		return nil, err
	}
	now := nowValue.Time()
	if verification.ExpiresAt == nil || !verification.ExpiresAt.After(now) {
		_, err = s.database.Model(contactVerificationTable).Ctx(ctx).
			Where(modeldo.ContactVerification{TenantId: notificationTenantID, VerificationNo: verificationNo}).
			Data(modeldo.ContactVerification{StatusCode: "expired"}).Update()
		return bizerr.NewCode(CodeVerificationExpired), err
	}
	if !hmac.Equal([]byte(verification.CodeDigest), []byte(codeDigest)) {
		attempts := verification.FailedAttempts + 1
		status := verification.StatusCode
		if attempts >= 5 {
			status = "locked"
		}
		_, err = s.database.Model(contactVerificationTable).Ctx(ctx).
			Where(modeldo.ContactVerification{TenantId: notificationTenantID, VerificationNo: verificationNo}).
			Data(modeldo.ContactVerification{FailedAttempts: attempts, StatusCode: status}).Update()
		return bizerr.NewCode(CodeVerificationInvalid), err
	}
	if verification.StatusCode != "sent" && verification.StatusCode != "result_unknown" {
		return bizerr.NewCode(CodeVerificationInvalid), nil
	}
	updated, err := s.database.Model(contactVerificationTable).Ctx(ctx).
		Where(modeldo.ContactVerification{TenantId: notificationTenantID, VerificationNo: verificationNo,
			StatusCode: verification.StatusCode, FailedAttempts: verification.FailedAttempts}).
		Data(modeldo.ContactVerification{StatusCode: "verified", VerifiedAt: &now}).Update()
	if err != nil || requireOne(updated) != nil {
		return nil, firstChannelIntentError(err, CodeConflict)
	}
	return nil, nil
}

func verificationMatchesBinding(verification entity.ContactVerification, binding *presalesBinding) bool {
	return binding != nil && verification.VerificationNo != "" && verification.PurposeCode == presalesChallengePurpose &&
		verification.OperationCode == binding.operation && verification.DestinationKind == binding.destinationKind &&
		verification.DestinationDigest == binding.destinationDigest && verification.BrowserDigest == binding.browserDigest &&
		verification.TicketDigest == binding.ticketDigest && verification.LocaleCode == binding.locale
}

func verificationIsTerminal(status string) bool {
	return status == "verified" || status == "locked" || status == "superseded"
}

func (s *serviceImpl) completePresalesOperation(ctx context.Context, operation *entity.PresalesOperation, binding *presalesBinding,
	ticketNo, accessToken string, expiresAt time.Time) (*PresalesAccess, error) {
	if operation == nil || binding == nil || !presalesTicketPattern.MatchString(ticketNo) || accessToken == "" {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	state := operation.LinkDeliveryState
	if operation.TicketNo != "" && operation.TicketNo != ticketNo {
		return nil, bizerr.NewCode(CodeConflict)
	}
	if state == "prepared" {
		state = "result_unknown"
	}
	if operation.LinkDeliveryState == "not_started" {
		if s.runtimeConfig.PublicCustomerOrigin == "" {
			state = "unavailable"
			if err := s.finishPresalesOperationWithoutSend(ctx, operation.OperationDigest, ticketNo, expiresAt, state); err != nil {
				return nil, err
			}
		} else {
			plan, planErr := s.loadPresalesDeliveryPlan(ctx, binding.destinationKind, presalesAccessLinkEvent, binding.locale,
				map[string]string{"ticket_no": ticketNo, "access_url": "https://example.invalid", "expires_at": expiresAt.UTC().Format(time.RFC3339)})
			if planErr != nil {
				state = "unavailable"
				if err := s.finishPresalesOperationWithoutSend(ctx, operation.OperationDigest, ticketNo, expiresAt, state); err != nil {
					return nil, err
				}
			} else {
				prepared, err := s.markPresalesLinkPrepared(ctx, operation.OperationDigest, ticketNo, expiresAt)
				if err != nil {
					return nil, err
				}
				if prepared {
					accessURL := buildPresalesAccessURL(s.runtimeConfig.PublicCustomerOrigin, accessToken, ticketNo)
					variables := map[string]string{"ticket_no": ticketNo, "access_url": accessURL,
						"expires_at": expiresAt.UTC().Format(time.RFC3339)}
					result := s.deliverPresalesOnce(ctx, plan, "plink_"+operation.OperationDigest[:32], presalesAccessLinkEvent,
						binding.destination, variables)
					state, _ = providerDeliveryState(result)
					if state == "failed" {
						state = "failed"
					}
					if err = s.finishPresalesLinkSend(ctx, operation.OperationDigest, state, result.CrossedExternalBorder); err != nil {
						return nil, err
					}
				} else {
					state = "result_unknown"
				}
			}
		}
	}
	return &PresalesAccess{TicketNo: ticketNo, AccessToken: accessToken, ExpiresAt: expiresAt.UTC(), LinkDeliveryState: state}, nil
}

func (s *serviceImpl) markPresalesLinkPrepared(ctx context.Context, operationDigest, ticketNo string, expiresAt time.Time) (bool, error) {
	result, err := s.database.Model(presalesOperationTable).Ctx(ctx).
		Where(modeldo.PresalesOperation{TenantId: notificationTenantID, OperationDigest: operationDigest,
			StatusCode: "prepared", LinkDeliveryState: "not_started"}).
		Data(modeldo.PresalesOperation{StatusCode: "core_completed", TicketNo: ticketNo,
			AccessExpiresAt: &expiresAt, LinkDeliveryState: "prepared"}).Update()
	if err != nil {
		return false, publicError(err)
	}
	affected, rowsErr := result.RowsAffected()
	return affected == 1, rowsErr
}

func (s *serviceImpl) finishPresalesOperationWithoutSend(ctx context.Context, operationDigest, ticketNo string, expiresAt time.Time, state string) error {
	result, err := s.database.Exec(ctx, `UPDATE plugin_idc_notify_presales_operation
SET status_code='completed', ticket_no=?, access_expires_at=?, link_delivery_state=?,
    completed_at=CURRENT_TIMESTAMP, updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=? AND operation_digest=? AND link_delivery_state='not_started'`,
		ticketNo, expiresAt, state, notificationTenantID, operationDigest)
	if err != nil {
		return publicError(err)
	}
	return publicError(requireOne(result))
}

func (s *serviceImpl) finishPresalesLinkSend(ctx context.Context, operationDigest, state string, crossed bool) error {
	result, err := s.database.Exec(ctx, `UPDATE plugin_idc_notify_presales_operation
SET status_code='completed', link_delivery_state=?, provider_boundary_crossed=?,
    completed_at=CURRENT_TIMESTAMP, updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=? AND operation_digest=? AND link_delivery_state='prepared'`,
		state, crossed, notificationTenantID, operationDigest)
	if err != nil {
		return publicError(err)
	}
	return publicError(requireOne(result))
}

func buildPresalesAccessURL(origin, token, ticketNo string) string {
	return strings.TrimSuffix(origin, "/") + "/auth/presales/access/" + url.PathEscape(token) + "?ticket=" + url.QueryEscape(ticketNo)
}

func (s *serviceImpl) loadPresalesOperation(ctx context.Context, operationDigest string) (*entity.PresalesOperation, error) {
	var row entity.PresalesOperation
	if err := s.database.Model(presalesOperationTable).Ctx(ctx).
		Where(modeldo.PresalesOperation{TenantId: notificationTenantID, OperationDigest: operationDigest}).Scan(&row); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if row.OperationDigest == "" {
		return nil, nil
	}
	return &row, nil
}
