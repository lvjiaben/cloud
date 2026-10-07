package billing

import (
	"context"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
)

type creditRestorationMovement struct {
	PreviousStatusCode string    `orm:"status_before_code"`
	RestoredStatusCode string    `orm:"status_after_code"`
	ReasonCode         string    `orm:"source_type_code"`
	SourceReference    string    `orm:"source_reference"`
	EvidenceDigest     string    `orm:"evidence_digest"`
	ActorID            int       `orm:"actor_id"`
	VersionBefore      int64     `orm:"account_version_before"`
	VersionAfter       int64     `orm:"account_version_after"`
	OccurredAt         time.Time `orm:"occurred_at"`
}

func (s *serviceImpl) RestoreCreditAccount(
	ctx context.Context,
	input CreditRestorationInput,
) (*CreditRestoration, error) {
	normalized, err := normalizeCreditRestoration(input)
	if err != nil {
		return nil, err
	}
	var result *CreditRestoration
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var transactionErr error
		result, transactionErr = s.restoreCreditAccount(ctx, normalized)
		return transactionErr
	})
	return creditRestorationResult(result, err)
}

func normalizeCreditRestoration(input CreditRestorationInput) (CreditRestorationInput, error) {
	if !exactlyTrimmed(
		input.CreditAccountNo, input.SourceReference, input.ReasonCode,
		input.EvidenceDigest,
	) {
		return CreditRestorationInput{}, bizerr.NewCode(CodeCreditInvalid)
	}
	input.CreditAccountNo = strings.TrimSpace(input.CreditAccountNo)
	input.SourceReference = strings.TrimSpace(input.SourceReference)
	input.ReasonCode = strings.TrimSpace(input.ReasonCode)
	input.EvidenceDigest = strings.TrimSpace(input.EvidenceDigest)
	if !validBusinessReference(input.CreditAccountNo, 40) ||
		input.ExpectedVersion <= 0 ||
		!validBusinessReference(input.SourceReference, 128) ||
		len(input.ReasonCode) > 32 ||
		!creditWriteoffReasonPattern.MatchString(input.ReasonCode) ||
		!canonicalDigestPattern.MatchString(input.EvidenceDigest) || input.ActorID <= 0 {
		return CreditRestorationInput{}, bizerr.NewCode(CodeCreditInvalid)
	}
	return input, nil
}

func (s *serviceImpl) restoreCreditAccount(
	ctx context.Context,
	input CreditRestorationInput,
) (*CreditRestoration, error) {
	account, err := loadCreditAccountByNo(ctx, s.database, input.CreditAccountNo, true)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, bizerr.NewCode(CodeCreditNotFound)
	}
	if !validCreditAccountProjection(account) {
		return nil, bizerr.NewCode(CodeCreditIntegrity)
	}
	existing, err := loadCreditRestorationMovement(
		ctx, s.database, account.CreditAccountNo, input.SourceReference,
	)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return projectCreditRestorationReplay(input, existing, account)
	}
	if account.Version != input.ExpectedVersion ||
		(account.StatusCode != "frozen" && account.StatusCode != "bad_debt") ||
		account.DelinquentPrincipalMinor != 0 || account.WrittenOffPrincipalMinor != 0 {
		return nil, bizerr.NewCode(CodeCreditConflict)
	}
	unresolved, err := countUnresolvedCreditDelinquencyCases(
		ctx, s.database, account.CreditAccountNo,
	)
	if err != nil {
		return nil, err
	}
	if unresolved != 0 {
		return nil, bizerr.NewCode(CodeCreditConflict)
	}
	targetStatus := "expired"
	if !account.Clock.Before(account.ValidFrom) && account.Clock.Before(account.ValidUntil) {
		targetStatus = "active"
	}
	previousStatus := account.StatusCode
	updated, err := s.database.Exec(ctx, `
UPDATE plugin_idc_core_credit_account
SET status_code = ?, freeze_reason_code = NULL, frozen_at = NULL,
    version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND credit_account_no = ? AND version = ?`,
		targetStatus, account.CreditAccountNo, account.Version)
	if err != nil {
		return nil, gerror.Wrap(err, "restore credit account")
	}
	if err = requireSingleCreditUpdate(updated); err != nil {
		return nil, err
	}
	if err = s.appendCreditMovement(ctx, creditMovementInput{
		MovementNo: deterministicCreditNo(
			"crm_", "restore", account.CreditAccountNo, input.SourceReference,
		),
		Account: account, MovementType: "unfreeze", AmountMinor: 0,
		StatusAfter: targetStatus, AvailableAfter: account.AvailableLimitMinor,
		UnbilledAfter:   account.UnbilledPrincipalMinor,
		BilledAfter:     account.BilledPrincipalMinor,
		DelinquentAfter: account.DelinquentPrincipalMinor,
		WrittenOffAfter: account.WrittenOffPrincipalMinor,
		SourceType:      input.ReasonCode, SourceReference: input.SourceReference,
		EvidenceDigest: input.EvidenceDigest, ActorType: "staff",
		ActorID: input.ActorID, OccurredAt: account.Clock,
	}); err != nil {
		return nil, err
	}
	account.StatusCode = targetStatus
	account.Version++
	return projectCreditRestoration(
		account, previousStatus, targetStatus, input.SourceReference,
		account.Clock, false,
	), nil
}

func loadCreditRestorationMovement(
	ctx context.Context,
	database gdb.DB,
	creditAccountNo string,
	sourceReference string,
) (*creditRestorationMovement, error) {
	var row *creditRestorationMovement
	if err := database.GetScan(ctx, &row, `
SELECT status_before_code, status_after_code, source_type_code,
       source_reference, evidence_digest, actor_id,
       account_version_before, account_version_after, occurred_at
FROM plugin_idc_core_credit_account_movement
WHERE tenant_id = 0 AND credit_account_no = ?
  AND movement_type_code = 'unfreeze' AND source_reference = ?`,
		creditAccountNo, sourceReference); err != nil {
		return nil, gerror.Wrap(err, "load credit restoration movement")
	}
	return row, nil
}

func countUnresolvedCreditDelinquencyCases(
	ctx context.Context,
	database gdb.DB,
	creditAccountNo string,
) (int64, error) {
	value, err := database.GetValue(ctx, `
SELECT COUNT(*)
FROM plugin_idc_core_credit_delinquency_case
WHERE tenant_id = 0 AND credit_account_no = ? AND status_code <> 'resolved'`,
		creditAccountNo)
	if err != nil {
		return 0, gerror.Wrap(err, "count unresolved credit delinquency cases")
	}
	return value.Int64(), nil
}

func projectCreditRestorationReplay(
	input CreditRestorationInput,
	movement *creditRestorationMovement,
	account *creditAccountRow,
) (*CreditRestoration, error) {
	if movement.VersionBefore != input.ExpectedVersion ||
		movement.VersionAfter != input.ExpectedVersion+1 ||
		movement.ReasonCode != input.ReasonCode ||
		movement.SourceReference != input.SourceReference ||
		movement.EvidenceDigest != input.EvidenceDigest ||
		movement.ActorID != input.ActorID {
		return nil, bizerr.NewCode(CodeCreditConflict)
	}
	return projectCreditRestoration(
		account, movement.PreviousStatusCode, movement.RestoredStatusCode,
		movement.SourceReference, movement.OccurredAt, true,
	), nil
}

func projectCreditRestoration(
	account *creditAccountRow,
	previousStatus string,
	restoredStatus string,
	sourceReference string,
	occurredAt time.Time,
	reused bool,
) *CreditRestoration {
	if account == nil {
		return nil
	}
	return &CreditRestoration{
		CreditAccountNo: account.CreditAccountNo, CustomerNo: account.CustomerNo,
		CurrencyCode: account.CurrencyCode, PreviousStatusCode: previousStatus,
		RestoredStatusCode: restoredStatus, CurrentStatusCode: account.StatusCode,
		AccountAvailableMinor:  account.AvailableLimitMinor,
		AccountDelinquentMinor: account.DelinquentPrincipalMinor,
		AccountWrittenOffMinor: account.WrittenOffPrincipalMinor,
		AccountVersion:         account.Version, SourceReference: sourceReference,
		OccurredAt: occurredAt, Reused: reused,
	}
}

func creditRestorationResult(
	result *CreditRestoration,
	err error,
) (*CreditRestoration, error) {
	if err == nil {
		if result == nil {
			return nil, bizerr.NewCode(CodeCreditIntegrity)
		}
		return result, nil
	}
	if _, ok := bizerr.As(err); ok {
		return nil, err
	}
	return nil, bizerr.WrapCode(err, CodeCreditUnavailable)
}
