// This file appends immutable legal-hold placement and release events.

package datalifecycle

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"strings"
)

// LegalHoldInput scopes one merchant-authorized legal-hold action.
type LegalHoldInput struct {
	DataClassCode       string
	AggregateRefHMAC    []byte
	ReasonCode          string
	EmployeePrincipalNo string
}

// LegalHoldResult identifies one immutable legal-hold event.
type LegalHoldResult struct {
	HoldNo      string
	HoldEventNo string
	ActionCode  string
}

// PlaceLegalHold appends one new active legal hold.
func (s *Service) PlaceLegalHold(ctx context.Context, input LegalHoldInput) (LegalHoldResult, error) {
	return s.appendLegalHold(ctx, "", "placed", input)
}

// ReleaseLegalHold closes one exact active hold without mutating its placement event.
func (s *Service) ReleaseLegalHold(
	ctx context.Context,
	holdNo string,
	input LegalHoldInput,
) (LegalHoldResult, error) {
	holdNo = strings.TrimSpace(holdNo)
	if len(holdNo) != 36 || !strings.HasPrefix(holdNo, "lhd_") {
		return LegalHoldResult{}, ErrInvalid
	}
	return s.appendLegalHold(ctx, holdNo, "released", input)
}

func (s *Service) appendLegalHold(
	ctx context.Context,
	holdNo string,
	action string,
	input LegalHoldInput,
) (holdResult LegalHoldResult, returnError error) {
	input.DataClassCode = strings.TrimSpace(input.DataClassCode)
	input.ReasonCode = strings.TrimSpace(input.ReasonCode)
	input.EmployeePrincipalNo = strings.TrimSpace(input.EmployeePrincipalNo)
	if !s.validContext(ctx) || !validDataClass(input.DataClassCode) ||
		!stableCodePattern.MatchString(input.ReasonCode) || !validEmployee(input.EmployeePrincipalNo) ||
		(len(input.AggregateRefHMAC) != 0 && len(input.AggregateRefHMAC) != sha256.Size) {
		return LegalHoldResult{}, ErrInvalid
	}
	transaction, err := s.database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return LegalHoldResult{}, ErrUnavailable
	}
	defer finishTransaction(transaction, &returnError)
	var policyEnabled bool
	if err = transaction.QueryRowContext(ctx, `
SELECT enabled FROM idc_gateway_data_lifecycle_policy
WHERE data_class_code = $1
FOR SHARE`, input.DataClassCode).Scan(&policyEnabled); err != nil || !policyEnabled {
		return LegalHoldResult{}, ErrConflict
	}
	if action == "placed" {
		holdNo, err = randomIdentifier(s.random, "lhd_")
		if err != nil {
			return LegalHoldResult{}, ErrUnavailable
		}
	} else {
		var placedClass string
		var placedRef []byte
		err = transaction.QueryRowContext(ctx, `
SELECT data_class_code, aggregate_ref_hmac
FROM idc_gateway_legal_hold_event
WHERE hold_no = $1 AND action_code = 'placed'
FOR SHARE`, holdNo).Scan(&placedClass, &placedRef)
		if err != nil || placedClass != input.DataClassCode ||
			!nullableDigestEqual(placedRef, input.AggregateRefHMAC) {
			return LegalHoldResult{}, ErrConflict
		}
		var alreadyReleased bool
		if err = transaction.QueryRowContext(ctx, `
SELECT EXISTS (
    SELECT 1 FROM idc_gateway_legal_hold_event
    WHERE hold_no = $1 AND action_code = 'released'
)`, holdNo).Scan(&alreadyReleased); err != nil || alreadyReleased {
			return LegalHoldResult{}, ErrConflict
		}
	}
	eventNo, err := randomIdentifier(s.random, "lhe_")
	if err != nil {
		return LegalHoldResult{}, ErrUnavailable
	}
	var aggregate any
	if len(input.AggregateRefHMAC) == sha256.Size {
		aggregate = input.AggregateRefHMAC
	}
	result, err := transaction.ExecContext(ctx, `
INSERT INTO idc_gateway_legal_hold_event
    (hold_event_no, hold_no, data_class_code, aggregate_ref_hmac,
     action_code, reason_code, employee_principal_no, occurred_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, CURRENT_TIMESTAMP)`,
		eventNo, holdNo, input.DataClassCode, aggregate, action,
		input.ReasonCode, input.EmployeePrincipalNo)
	if err != nil || rowsAffected(result) != 1 {
		return LegalHoldResult{}, classifyDatabaseError(err)
	}
	if err = transaction.Commit(); err != nil {
		return LegalHoldResult{}, classifyDatabaseError(err)
	}
	return LegalHoldResult{HoldNo: holdNo, HoldEventNo: eventNo, ActionCode: action}, nil
}

func nullableDigestEqual(left, right []byte) bool {
	if len(left) == 0 && len(right) == 0 {
		return true
	}
	return equalDigest(left, right)
}
