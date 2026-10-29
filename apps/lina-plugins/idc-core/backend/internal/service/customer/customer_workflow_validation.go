// This file normalizes customer workflow commands and validates injected
// cross-domain projections before any workflow persistence is attempted.

package customer

import (
	"encoding/json"
	"sort"
	"strings"
	"time"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/reauth"
)

const (
	closurePrecheckContractVersion = 1
	maximumClosureFindingCount     = 1_000_000_000
	maximumClosurePrecheckAge      = 5 * time.Minute
	closureRetentionPolicy         = "customer_retention_v1"
	closureAnonymizationPolicy     = "customer_anonymization_v1"
	dataExportTaskType             = "customer.data_export"
	dataExportTaskSubjectType      = "customer"
)

type normalizedLegalChange struct {
	customerNo             string
	reason                 LegalChangeReason
	changeLegalName        bool
	changeLegalIdentifier  bool
	expectedProfileVersion int64
	idempotencyKey         string
}

type legalChangeFingerprint struct {
	Version                int               `json:"version"`
	CustomerNo             string            `json:"customer_no"`
	Reason                 LegalChangeReason `json:"reason"`
	ChangeLegalName        bool              `json:"change_legal_name"`
	ChangeLegalIdentifier  bool              `json:"change_legal_identifier"`
	ExpectedProfileVersion int64             `json:"expected_profile_version"`
}

func normalizeLegalChange(input RequestOwnLegalChangeInput) (normalizedLegalChange, error) {
	idempotencyKey := strings.TrimSpace(input.IdempotencyKey)
	result := normalizedLegalChange{
		customerNo:             input.CustomerNo,
		reason:                 input.Reason,
		expectedProfileVersion: input.ExpectedProfileVersion,
		idempotencyKey:         idempotencyKey,
	}
	if !validCustomerNo(input.CustomerNo) || !validLegalChangeReason(input.Reason) ||
		input.ExpectedProfileVersion <= 0 ||
		!validBoundedText(idempotencyKey, maximumIdempotencyKeyRunes) ||
		len(input.Fields) == 0 || len(input.Fields) > 2 {
		return normalizedLegalChange{}, bizerr.NewCode(CodeWorkflowInvalid)
	}
	for _, field := range input.Fields {
		switch field {
		case LegalChangeFieldName:
			if result.changeLegalName {
				return normalizedLegalChange{}, bizerr.NewCode(CodeWorkflowInvalid)
			}
			result.changeLegalName = true
		case LegalChangeFieldIdentifier:
			if result.changeLegalIdentifier {
				return normalizedLegalChange{}, bizerr.NewCode(CodeWorkflowInvalid)
			}
			result.changeLegalIdentifier = true
		default:
			return normalizedLegalChange{}, bizerr.NewCode(CodeWorkflowInvalid)
		}
	}
	return result, nil
}

func validLegalChangeReason(reason LegalChangeReason) bool {
	switch reason {
	case LegalChangeReasonNameChange, LegalChangeReasonIdentifierCorrection,
		LegalChangeReasonEnterpriseRegistration, LegalChangeReasonOther:
		return true
	default:
		return false
	}
}

func (input normalizedLegalChange) fingerprint() (string, error) {
	document, err := json.Marshal(legalChangeFingerprint{
		Version:                1,
		CustomerNo:             input.customerNo,
		Reason:                 input.reason,
		ChangeLegalName:        input.changeLegalName,
		ChangeLegalIdentifier:  input.changeLegalIdentifier,
		ExpectedProfileVersion: input.expectedProfileVersion,
	})
	if err != nil {
		return "", err
	}
	return digest("customer-legal-change-request-v1", string(document)), nil
}

type normalizedAccountTransition struct {
	customerNo      string
	toStatus        Status
	reason          AccountTransitionReason
	expectedVersion int64
	idempotencyKey  string
}

type accountTransitionFingerprint struct {
	Version         int                     `json:"version"`
	CustomerNo      string                  `json:"customer_no"`
	ToStatus        Status                  `json:"to_status"`
	Reason          AccountTransitionReason `json:"reason"`
	ExpectedVersion int64                   `json:"expected_version"`
}

func normalizeAccountTransition(input TransitionAccountInput) (normalizedAccountTransition, error) {
	idempotencyKey := strings.TrimSpace(input.IdempotencyKey)
	if !validCustomerNo(input.CustomerNo) || !validStatus(input.ToStatus) ||
		input.ToStatus == StatusAnonymized || !validAccountTransitionReason(input.Reason) ||
		input.ExpectedVersion <= 0 || !validBoundedText(idempotencyKey, maximumIdempotencyKeyRunes) {
		return normalizedAccountTransition{}, bizerr.NewCode(CodeWorkflowInvalid)
	}
	return normalizedAccountTransition{
		customerNo: input.CustomerNo, toStatus: input.ToStatus, reason: input.Reason,
		expectedVersion: input.ExpectedVersion, idempotencyKey: idempotencyKey,
	}, nil
}

func validAccountTransitionReason(reason AccountTransitionReason) bool {
	switch reason {
	case AccountReasonActivationCompleted, AccountReasonVerificationRequired,
		AccountReasonRiskRestriction, AccountReasonRestrictionResolved,
		AccountReasonManualDisable, AccountReasonAccountReactivated:
		return true
	default:
		return false
	}
}

func validAccountTransition(from Status, input normalizedAccountTransition) bool {
	switch {
	case from == StatusPendingActivation && input.toStatus == StatusActive:
		return input.reason == AccountReasonActivationCompleted
	case from == StatusPendingActivation && input.toStatus == StatusRestricted:
		return input.reason == AccountReasonVerificationRequired
	case from == StatusPendingActivation && input.toStatus == StatusDisabled:
		return input.reason == AccountReasonManualDisable
	case from == StatusActive && input.toStatus == StatusRestricted:
		return input.reason == AccountReasonRiskRestriction || input.reason == AccountReasonVerificationRequired
	case from == StatusActive && input.toStatus == StatusDisabled:
		return input.reason == AccountReasonManualDisable
	case from == StatusRestricted && input.toStatus == StatusActive:
		return input.reason == AccountReasonRestrictionResolved
	case from == StatusRestricted && input.toStatus == StatusDisabled:
		return input.reason == AccountReasonManualDisable
	case from == StatusDisabled && input.toStatus == StatusActive:
		return input.reason == AccountReasonAccountReactivated
	default:
		return false
	}
}

func (input normalizedAccountTransition) fingerprint() (string, error) {
	document, err := json.Marshal(accountTransitionFingerprint{
		Version: 1, CustomerNo: input.customerNo, ToStatus: input.toStatus,
		Reason: input.reason, ExpectedVersion: input.expectedVersion,
	})
	if err != nil {
		return "", err
	}
	return digest("customer-account-transition-v1", string(document)), nil
}

type normalizedClosureRequest struct {
	customerNo      string
	expectedVersion int64
	idempotencyKey  string
}

type closureFingerprint struct {
	Version         int    `json:"version"`
	CustomerNo      string `json:"customer_no"`
	ExpectedVersion int64  `json:"expected_version"`
	Confirmed       bool   `json:"confirmed"`
}

func normalizeClosureRequest(input RequestOwnClosureInput) (normalizedClosureRequest, error) {
	idempotencyKey := strings.TrimSpace(input.IdempotencyKey)
	if !validCustomerNo(input.CustomerNo) || input.ExpectedVersion <= 0 || !input.Confirmed ||
		!validBoundedText(idempotencyKey, maximumIdempotencyKeyRunes) {
		return normalizedClosureRequest{}, bizerr.NewCode(CodeWorkflowInvalid)
	}
	return normalizedClosureRequest{
		customerNo: input.CustomerNo, expectedVersion: input.ExpectedVersion,
		idempotencyKey: idempotencyKey,
	}, nil
}

func (input normalizedClosureRequest) fingerprint() (string, error) {
	document, err := json.Marshal(closureFingerprint{
		Version: 1, CustomerNo: input.customerNo,
		ExpectedVersion: input.expectedVersion, Confirmed: true,
	})
	if err != nil {
		return "", err
	}
	return digest("customer-closure-request-v1", string(document)), nil
}

func validateClosureBatch(
	customerNo string,
	results []ClosurePrecheckResult,
	now time.Time,
) (ClosurePrecheckResult, error) {
	if len(results) != 1 || results[0].CustomerNo != customerNo {
		return ClosurePrecheckResult{}, bizerr.NewCode(CodeWorkflowUnavailable)
	}
	result := results[0]
	result.Findings = append([]ClosureFinding(nil), results[0].Findings...)
	checkedAt := databaseTime(result.CheckedAt)
	current := databaseTime(now)
	if result.ContractVersion != closurePrecheckContractVersion || checkedAt.IsZero() ||
		checkedAt.Before(current.Add(-maximumClosurePrecheckAge)) ||
		checkedAt.After(current) || len(result.Findings) > 6 {
		return ClosurePrecheckResult{}, bizerr.NewCode(CodeWorkflowUnavailable)
	}
	seen := make(map[ClosureFindingCode]struct{}, len(result.Findings))
	for _, finding := range result.Findings {
		if !validClosureFindingCode(finding.Code) || finding.Count <= 0 ||
			finding.Count > maximumClosureFindingCount {
			return ClosurePrecheckResult{}, bizerr.NewCode(CodeWorkflowUnavailable)
		}
		if _, duplicate := seen[finding.Code]; duplicate {
			return ClosurePrecheckResult{}, bizerr.NewCode(CodeWorkflowUnavailable)
		}
		seen[finding.Code] = struct{}{}
	}
	result.CheckedAt = checkedAt
	sort.Slice(result.Findings, func(left, right int) bool {
		return result.Findings[left].Code < result.Findings[right].Code
	})
	return result, nil
}

func validClosureFindingCode(code ClosureFindingCode) bool {
	switch code {
	case ClosureFindingUnsettledFunds, ClosureFindingActiveServices,
		ClosureFindingUnsettledInvoices,
		ClosureFindingInProgressOperations, ClosureFindingLegalRetention:
		return true
	default:
		return false
	}
}

func closureHasHardBlocker(findings []ClosureFinding) bool {
	for _, finding := range findings {
		if finding.Code != ClosureFindingLegalRetention && finding.Count > 0 {
			return true
		}
	}
	return false
}

type normalizedDataExport struct {
	customerNo      string
	scope           DataExportScope
	format          DataExportFormat
	expectedVersion int64
	idempotencyKey  string
}

type dataExportFingerprint struct {
	Version         int              `json:"version"`
	CustomerNo      string           `json:"customer_no"`
	Scope           DataExportScope  `json:"scope"`
	Format          DataExportFormat `json:"format"`
	ExpectedVersion int64            `json:"expected_version"`
	Confirmed       bool             `json:"confirmed"`
}

func normalizeDataExport(input OwnDataExportIntent) (normalizedDataExport, error) {
	idempotencyKey := strings.TrimSpace(input.IdempotencyKey)
	if !validCustomerNo(input.CustomerNo) || input.Scope != DataExportScopeOwnDeliverable ||
		input.Format != DataExportFormatJSONZip || input.ExpectedVersion <= 0 || !input.Confirmed ||
		!validBoundedText(idempotencyKey, maximumIdempotencyKeyRunes) {
		return normalizedDataExport{}, bizerr.NewCode(CodeWorkflowInvalid)
	}
	return normalizedDataExport{
		customerNo: input.CustomerNo, scope: input.Scope, format: input.Format,
		expectedVersion: input.ExpectedVersion, idempotencyKey: idempotencyKey,
	}, nil
}

func (input normalizedDataExport) fingerprint() (string, error) {
	document, err := json.Marshal(dataExportFingerprint{
		Version: 1, CustomerNo: input.customerNo, Scope: input.scope,
		Format: input.format, ExpectedVersion: input.expectedVersion, Confirmed: true,
	})
	if err != nil {
		return "", err
	}
	return digest("customer-data-export-request-v1", string(document)), nil
}

func (input normalizedDataExport) binding() (reauth.Binding, error) {
	fingerprint, err := input.fingerprint()
	if err != nil {
		return reauth.Binding{}, err
	}
	// The idempotency digest is part of the immutable operation preview so one
	// proof cannot authorize the same export under a different replay scope.
	target := digest(
		"customer-data-export-reauth-target-v1",
		fingerprint+"\x00"+digest("customer-data-export-idempotency-v1", input.idempotencyKey),
	)
	return reauth.Binding{Purpose: reauth.PurposeSensitiveExport, TargetDigest: target}, nil
}
