package billing

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
)

type creditAccountRow struct {
	CreditAccountNo          string    `orm:"credit_account_no"`
	CustomerNo               string    `orm:"customer_no"`
	CurrencyCode             string    `orm:"currency_code"`
	CurrentTermsNo           string    `orm:"current_terms_no"`
	CurrentTermsVersion      int64     `orm:"current_terms_version"`
	StatusCode               string    `orm:"status_code"`
	TotalLimitMinor          int64     `orm:"total_limit_minor"`
	AvailableLimitMinor      int64     `orm:"available_limit_minor"`
	UnbilledPrincipalMinor   int64     `orm:"unbilled_principal_minor"`
	BilledPrincipalMinor     int64     `orm:"billed_principal_minor"`
	DelinquentPrincipalMinor int64     `orm:"delinquent_principal_minor"`
	WrittenOffPrincipalMinor int64     `orm:"written_off_principal_minor"`
	ValidFrom                time.Time `orm:"valid_from"`
	ValidUntil               time.Time `orm:"valid_until"`
	Version                  int64     `orm:"version"`
	Usable                   bool      `orm:"usable"`
	Clock                    time.Time `orm:"clock"`
}

type creditExposureRow struct {
	ExposureNo               string `orm:"exposure_no"`
	CreditAccountNo          string `orm:"credit_account_no"`
	CustomerNo               string `orm:"customer_no"`
	CurrencyCode             string `orm:"currency_code"`
	SourceType               string `orm:"source_type_code"`
	SourceReference          string `orm:"source_reference"`
	OrderNo                  string `orm:"order_no"`
	InvoiceNo                string `orm:"invoice_no"`
	OriginalPrincipalMinor   int64  `orm:"original_principal_minor"`
	UnbilledPrincipalMinor   int64  `orm:"unbilled_principal_minor"`
	BilledPrincipalMinor     int64  `orm:"billed_principal_minor"`
	RepaidPrincipalMinor     int64  `orm:"repaid_principal_minor"`
	ReleasedPrincipalMinor   int64  `orm:"released_principal_minor"`
	WrittenOffPrincipalMinor int64  `orm:"written_off_principal_minor"`
	StatusCode               string `orm:"status_code"`
	SourceSnapshotDigest     string `orm:"source_snapshot_digest"`
	ContentDigest            string `orm:"content_digest"`
	Version                  int64  `orm:"version"`
}

type creditMovementInput struct {
	MovementNo      string
	Account         *creditAccountRow
	ExposureNo      string
	StatementNo     string
	RepaymentNo     string
	MovementType    string
	AmountMinor     int64
	StatusAfter     string
	AvailableAfter  int64
	UnbilledAfter   int64
	BilledAfter     int64
	DelinquentAfter int64
	WrittenOffAfter int64
	SourceType      string
	SourceReference string
	EvidenceDigest  string
	ActorType       string
	ActorID         int
	OccurredAt      time.Time
}

func (s *serviceImpl) ReserveCredit(
	ctx context.Context,
	input CreditReserveInput,
) (*CreditExposure, error) {
	normalized, err := normalizeCreditReserve(input)
	if err != nil {
		return nil, err
	}
	var result *CreditExposure
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		account, loadErr := loadCreditAccountByOwner(
			ctx, s.database, normalized.CustomerNo, normalized.CurrencyCode, true,
		)
		if loadErr != nil {
			return loadErr
		}
		if account == nil {
			return bizerr.NewCode(CodeCreditNotFound)
		}
		existing, loadErr := loadCreditExposureBySource(
			ctx, s.database, account.CreditAccountNo,
			normalized.SourceType, normalized.SourceReference, false,
		)
		if loadErr != nil {
			return loadErr
		}
		if existing != nil {
			if existing.OriginalPrincipalMinor != normalized.AmountMinor ||
				existing.SourceSnapshotDigest != normalized.SourceSnapshotDigest ||
				existing.OrderNo != normalized.OrderNo || existing.InvoiceNo != normalized.InvoiceNo {
				return bizerr.NewCode(CodeCreditConflict)
			}
			result = projectCreditExposure(existing, account, true)
			return nil
		}
		if customerErr := s.requireActiveCustomer(ctx, normalized.CustomerNo); customerErr != nil {
			return customerErr
		}
		if !validCreditAccountProjection(account) || !account.Usable ||
			account.StatusCode != "active" || account.AvailableLimitMinor < normalized.AmountMinor {
			return bizerr.NewCode(CodeCreditConflict)
		}
		exposureNo := deterministicCreditNo(
			"cre_", "exposure", account.CreditAccountNo,
			normalized.SourceType, normalized.SourceReference,
		)
		exposureDigest, digestErr := contentDigest(struct {
			ExposureNo, CreditAccountNo, CustomerNo, CurrencyCode string
			SourceType, SourceReference, OrderNo, InvoiceNo       string
			AmountMinor                                           int64
			SourceSnapshotDigest                                  string
		}{
			exposureNo, account.CreditAccountNo, normalized.CustomerNo,
			normalized.CurrencyCode, normalized.SourceType,
			normalized.SourceReference, normalized.OrderNo, normalized.InvoiceNo,
			normalized.AmountMinor, normalized.SourceSnapshotDigest,
		})
		if digestErr != nil {
			return digestErr
		}
		if _, insertErr := s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_credit_exposure (
    tenant_id, exposure_no, credit_account_no, customer_no, currency_code,
    source_type_code, source_reference, order_no, invoice_no,
    original_principal_minor, unbilled_principal_minor, status_code,
    source_snapshot_digest, content_digest, version
) VALUES (0, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'unbilled', ?, ?, 1)`,
			exposureNo, account.CreditAccountNo, normalized.CustomerNo,
			normalized.CurrencyCode, normalized.SourceType,
			normalized.SourceReference, nullableCreditReference(normalized.OrderNo),
			nullableCreditReference(normalized.InvoiceNo), normalized.AmountMinor,
			normalized.AmountMinor, normalized.SourceSnapshotDigest,
			exposureDigest); insertErr != nil {
			return gerror.Wrap(insertErr, "create credit exposure")
		}
		afterAvailable := account.AvailableLimitMinor - normalized.AmountMinor
		afterUnbilled := account.UnbilledPrincipalMinor + normalized.AmountMinor
		updated, updateErr := s.database.Exec(ctx, `
UPDATE plugin_idc_core_credit_account
SET available_limit_minor = ?, unbilled_principal_minor = ?,
    version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND credit_account_no = ? AND version = ?
  AND status_code = 'active'`, afterAvailable, afterUnbilled,
			account.CreditAccountNo, account.Version)
		if updateErr != nil {
			return gerror.Wrap(updateErr, "reserve credit account limit")
		}
		rows, rowsErr := updated.RowsAffected()
		if rowsErr != nil || rows != 1 {
			return bizerr.NewCode(CodeCreditConflict)
		}
		movementNo := deterministicCreditNo(
			"crm_", "reserve", account.CreditAccountNo,
			normalized.SourceType, normalized.SourceReference,
		)
		if movementErr := s.appendCreditMovement(ctx, creditMovementInput{
			MovementNo: movementNo, Account: account, ExposureNo: exposureNo,
			MovementType: "reserve", AmountMinor: normalized.AmountMinor,
			StatusAfter: account.StatusCode, AvailableAfter: afterAvailable,
			UnbilledAfter: afterUnbilled, BilledAfter: account.BilledPrincipalMinor,
			DelinquentAfter: account.DelinquentPrincipalMinor,
			WrittenOffAfter: account.WrittenOffPrincipalMinor,
			SourceType:      normalized.SourceType,
			SourceReference: normalized.SourceReference,
			EvidenceDigest:  normalized.SourceSnapshotDigest,
			ActorType:       normalized.ActorType, ActorID: normalized.ActorID,
			OccurredAt: account.Clock,
		}); movementErr != nil {
			return movementErr
		}
		account.AvailableLimitMinor = afterAvailable
		account.UnbilledPrincipalMinor = afterUnbilled
		account.Version++
		exposure, loadErr := loadCreditExposureByNo(ctx, s.database, exposureNo, false)
		if loadErr != nil {
			return loadErr
		}
		if exposure == nil {
			return bizerr.NewCode(CodeCreditIntegrity)
		}
		result = projectCreditExposure(exposure, account, false)
		return nil
	})
	return creditExposureResult(result, err)
}

func (s *serviceImpl) ReleaseUnbilledCredit(
	ctx context.Context,
	input CreditReleaseInput,
) (*CreditExposure, error) {
	normalized, err := normalizeCreditRelease(input)
	if err != nil {
		return nil, err
	}
	var result *CreditExposure
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		identity, loadErr := loadCreditExposureByNo(ctx, s.database, normalized.ExposureNo, false)
		if loadErr != nil {
			return loadErr
		}
		if identity == nil {
			return bizerr.NewCode(CodeCreditNotFound)
		}
		account, loadErr := loadCreditAccountByNo(ctx, s.database, identity.CreditAccountNo, true)
		if loadErr != nil {
			return loadErr
		}
		if account == nil || !validCreditAccountProjection(account) {
			return bizerr.NewCode(CodeCreditIntegrity)
		}
		exposure, loadErr := loadCreditExposureByNo(ctx, s.database, normalized.ExposureNo, true)
		if loadErr != nil {
			return loadErr
		}
		if exposure == nil || exposure.CreditAccountNo != account.CreditAccountNo {
			return bizerr.NewCode(CodeCreditIntegrity)
		}
		var existingMovement *struct {
			ExposureNo     string `orm:"exposure_no"`
			AmountMinor    int64  `orm:"amount_minor"`
			EvidenceDigest string `orm:"evidence_digest"`
		}
		if loadErr = s.database.GetScan(ctx, &existingMovement, `
SELECT COALESCE(exposure_no, '') AS exposure_no, amount_minor, evidence_digest
FROM plugin_idc_core_credit_account_movement
WHERE tenant_id = 0 AND credit_account_no = ?
  AND source_type_code = 'release' AND source_reference = ?`,
			account.CreditAccountNo, normalized.SourceReference); loadErr != nil {
			return gerror.Wrap(loadErr, "load existing credit release")
		}
		if existingMovement != nil {
			if existingMovement.ExposureNo != exposure.ExposureNo ||
				existingMovement.AmountMinor != exposure.ReleasedPrincipalMinor ||
				existingMovement.EvidenceDigest != normalized.EvidenceDigest {
				return bizerr.NewCode(CodeCreditConflict)
			}
			result = projectCreditExposure(exposure, account, true)
			return nil
		}
		if exposure.StatusCode != "unbilled" ||
			exposure.UnbilledPrincipalMinor != exposure.OriginalPrincipalMinor ||
			exposure.BilledPrincipalMinor != 0 || exposure.RepaidPrincipalMinor != 0 ||
			exposure.ReleasedPrincipalMinor != 0 || exposure.WrittenOffPrincipalMinor != 0 ||
			account.UnbilledPrincipalMinor < exposure.UnbilledPrincipalMinor {
			return bizerr.NewCode(CodeCreditConflict)
		}
		amount := exposure.UnbilledPrincipalMinor
		if _, updateErr := s.database.Exec(ctx, `
UPDATE plugin_idc_core_credit_exposure
SET unbilled_principal_minor = 0, released_principal_minor = ?,
    status_code = 'released', version = version + 1,
    updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND exposure_no = ? AND version = ?`,
			amount, exposure.ExposureNo, exposure.Version); updateErr != nil {
			return gerror.Wrap(updateErr, "release unbilled credit exposure")
		}
		afterAvailable := account.AvailableLimitMinor + amount
		afterUnbilled := account.UnbilledPrincipalMinor - amount
		updated, updateErr := s.database.Exec(ctx, `
UPDATE plugin_idc_core_credit_account
SET available_limit_minor = ?, unbilled_principal_minor = ?,
    version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND credit_account_no = ? AND version = ?`,
			afterAvailable, afterUnbilled, account.CreditAccountNo, account.Version)
		if updateErr != nil {
			return gerror.Wrap(updateErr, "restore released credit limit")
		}
		rows, rowsErr := updated.RowsAffected()
		if rowsErr != nil || rows != 1 {
			return bizerr.NewCode(CodeCreditConflict)
		}
		movementNo := deterministicCreditNo(
			"crm_", "release", account.CreditAccountNo, normalized.SourceReference,
		)
		if movementErr := s.appendCreditMovement(ctx, creditMovementInput{
			MovementNo: movementNo, Account: account, ExposureNo: exposure.ExposureNo,
			MovementType: "release", AmountMinor: amount,
			StatusAfter: account.StatusCode, AvailableAfter: afterAvailable,
			UnbilledAfter: afterUnbilled, BilledAfter: account.BilledPrincipalMinor,
			DelinquentAfter: account.DelinquentPrincipalMinor,
			WrittenOffAfter: account.WrittenOffPrincipalMinor,
			SourceType:      "release", SourceReference: normalized.SourceReference,
			EvidenceDigest: normalized.EvidenceDigest,
			ActorType:      normalized.ActorType, ActorID: normalized.ActorID,
			OccurredAt: account.Clock,
		}); movementErr != nil {
			return movementErr
		}
		account.AvailableLimitMinor = afterAvailable
		account.UnbilledPrincipalMinor = afterUnbilled
		account.Version++
		exposure.UnbilledPrincipalMinor = 0
		exposure.ReleasedPrincipalMinor = amount
		exposure.StatusCode = "released"
		exposure.Version++
		result = projectCreditExposure(exposure, account, false)
		return nil
	})
	return creditExposureResult(result, err)
}

func (s *serviceImpl) appendCreditMovement(ctx context.Context, input creditMovementInput) error {
	if input.Account == nil {
		return bizerr.NewCode(CodeCreditIntegrity)
	}
	idempotencyDigest, err := contentDigest(struct {
		CreditAccountNo, MovementType, SourceType, SourceReference string
	}{
		input.Account.CreditAccountNo, input.MovementType,
		input.SourceType, input.SourceReference,
	})
	if err != nil {
		return err
	}
	movementDigest, err := contentDigest(struct {
		MovementNo, CreditAccountNo, ExposureNo, StatementNo, RepaymentNo    string
		MovementType, StatusBefore, StatusAfter, SourceType, SourceReference string
		AmountMinor, VersionBefore, VersionAfter                             int64
		AvailableBefore, AvailableAfter, UnbilledBefore, UnbilledAfter       int64
		BilledBefore, BilledAfter, DelinquentBefore, DelinquentAfter         int64
		WrittenOffBefore, WrittenOffAfter                                    int64
		EvidenceDigest, ActorType                                            string
		ActorID                                                              int
	}{
		input.MovementNo, input.Account.CreditAccountNo, input.ExposureNo,
		input.StatementNo, input.RepaymentNo, input.MovementType,
		input.Account.StatusCode, input.StatusAfter, input.SourceType,
		input.SourceReference, input.AmountMinor, input.Account.Version,
		input.Account.Version + 1, input.Account.AvailableLimitMinor,
		input.AvailableAfter, input.Account.UnbilledPrincipalMinor,
		input.UnbilledAfter, input.Account.BilledPrincipalMinor,
		input.BilledAfter, input.Account.DelinquentPrincipalMinor,
		input.DelinquentAfter, input.Account.WrittenOffPrincipalMinor,
		input.WrittenOffAfter, input.EvidenceDigest, input.ActorType,
		input.ActorID,
	})
	if err != nil {
		return err
	}
	_, err = s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_credit_account_movement (
    tenant_id, movement_no, credit_account_no, customer_no, currency_code,
    exposure_no, statement_no, repayment_no, movement_type_code, amount_minor,
    account_version_before, account_version_after, status_before_code,
    status_after_code, total_limit_before_minor, total_limit_after_minor,
    available_before_minor, available_after_minor, unbilled_before_minor,
    unbilled_after_minor, billed_before_minor, billed_after_minor,
    delinquent_before_minor, delinquent_after_minor, written_off_before_minor,
    written_off_after_minor, source_type_code, source_reference,
    idempotency_digest, evidence_digest, content_digest, actor_type_code,
    actor_id, occurred_at
) VALUES (
    0, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
    ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
)`, input.MovementNo, input.Account.CreditAccountNo, input.Account.CustomerNo,
		input.Account.CurrencyCode, nullableCreditReference(input.ExposureNo),
		nullableCreditReference(input.StatementNo), nullableCreditReference(input.RepaymentNo),
		input.MovementType, input.AmountMinor, input.Account.Version,
		input.Account.Version+1, input.Account.StatusCode, input.StatusAfter,
		input.Account.TotalLimitMinor, input.Account.TotalLimitMinor,
		input.Account.AvailableLimitMinor, input.AvailableAfter,
		input.Account.UnbilledPrincipalMinor, input.UnbilledAfter,
		input.Account.BilledPrincipalMinor, input.BilledAfter,
		input.Account.DelinquentPrincipalMinor, input.DelinquentAfter,
		input.Account.WrittenOffPrincipalMinor, input.WrittenOffAfter,
		input.SourceType, input.SourceReference, idempotencyDigest,
		input.EvidenceDigest, movementDigest, input.ActorType, input.ActorID,
		input.OccurredAt)
	if err != nil {
		return gerror.Wrap(err, "append credit account movement")
	}
	return nil
}

func loadCreditAccountByOwner(
	ctx context.Context,
	database gdb.DB,
	customerNo string,
	currencyCode string,
	lock bool,
) (*creditAccountRow, error) {
	return loadCreditAccount(ctx, database,
		"account.customer_no = ? AND account.currency_code = ?", lock,
		customerNo, currencyCode)
}

func loadCreditAccountByNo(
	ctx context.Context,
	database gdb.DB,
	creditAccountNo string,
	lock bool,
) (*creditAccountRow, error) {
	return loadCreditAccount(ctx, database, "account.credit_account_no = ?", lock, creditAccountNo)
}

func loadCreditAccount(
	ctx context.Context,
	database gdb.DB,
	predicate string,
	lock bool,
	arguments ...any,
) (*creditAccountRow, error) {
	locking := ""
	if lock {
		locking = " FOR UPDATE"
	}
	var row *creditAccountRow
	query := `
SELECT account.credit_account_no, account.customer_no, account.currency_code,
       account.current_terms_no, account.current_terms_version,
       account.status_code, account.total_limit_minor,
       account.available_limit_minor, account.unbilled_principal_minor,
       account.billed_principal_minor, account.delinquent_principal_minor,
       account.written_off_principal_minor, account.valid_from,
       account.valid_until, account.version,
       (account.status_code = 'active'
        AND CURRENT_TIMESTAMP >= account.valid_from
        AND CURRENT_TIMESTAMP < account.valid_until) AS usable,
       CURRENT_TIMESTAMP AS clock
FROM plugin_idc_core_credit_account account
WHERE account.tenant_id = 0 AND ` + predicate + locking
	if err := database.GetScan(ctx, &row, query, arguments...); err != nil {
		return nil, gerror.Wrap(err, "load credit account")
	}
	return row, nil
}

func loadCreditExposureBySource(
	ctx context.Context,
	database gdb.DB,
	creditAccountNo string,
	sourceType string,
	sourceReference string,
	lock bool,
) (*creditExposureRow, error) {
	return loadCreditExposure(ctx, database,
		"exposure.credit_account_no = ? AND exposure.source_type_code = ? AND exposure.source_reference = ?",
		lock, creditAccountNo, sourceType, sourceReference)
}

func loadCreditExposureByNo(
	ctx context.Context,
	database gdb.DB,
	exposureNo string,
	lock bool,
) (*creditExposureRow, error) {
	return loadCreditExposure(ctx, database, "exposure.exposure_no = ?", lock, exposureNo)
}

func loadCreditExposure(
	ctx context.Context,
	database gdb.DB,
	predicate string,
	lock bool,
	arguments ...any,
) (*creditExposureRow, error) {
	locking := ""
	if lock {
		locking = " FOR UPDATE"
	}
	var row *creditExposureRow
	query := `
SELECT exposure.exposure_no, exposure.credit_account_no, exposure.customer_no,
       exposure.currency_code, exposure.source_type_code,
       exposure.source_reference, COALESCE(exposure.order_no, '') AS order_no,
       COALESCE(exposure.invoice_no, '') AS invoice_no,
       exposure.original_principal_minor, exposure.unbilled_principal_minor,
       exposure.billed_principal_minor, exposure.repaid_principal_minor,
       exposure.released_principal_minor, exposure.written_off_principal_minor,
       exposure.status_code, exposure.source_snapshot_digest,
       exposure.content_digest, exposure.version
FROM plugin_idc_core_credit_exposure exposure
WHERE exposure.tenant_id = 0 AND ` + predicate + locking
	if err := database.GetScan(ctx, &row, query, arguments...); err != nil {
		return nil, gerror.Wrap(err, "load credit exposure")
	}
	return row, nil
}

func validCreditAccountProjection(account *creditAccountRow) bool {
	return account != nil && account.Version > 0 && account.TotalLimitMinor > 0 &&
		account.AvailableLimitMinor >= 0 && account.UnbilledPrincipalMinor >= 0 &&
		account.BilledPrincipalMinor >= 0 && account.DelinquentPrincipalMinor >= 0 &&
		account.WrittenOffPrincipalMinor >= 0 &&
		account.DelinquentPrincipalMinor <= account.BilledPrincipalMinor &&
		account.TotalLimitMinor == account.AvailableLimitMinor+
			account.UnbilledPrincipalMinor+account.BilledPrincipalMinor+
			account.WrittenOffPrincipalMinor
}

func projectCreditExposure(
	exposure *creditExposureRow,
	account *creditAccountRow,
	reused bool,
) *CreditExposure {
	if exposure == nil || account == nil {
		return nil
	}
	return &CreditExposure{
		ExposureNo: exposure.ExposureNo, CreditAccountNo: exposure.CreditAccountNo,
		CustomerNo: exposure.CustomerNo, CurrencyCode: exposure.CurrencyCode,
		SourceType: exposure.SourceType, SourceReference: exposure.SourceReference,
		OriginalPrincipalMinor:   exposure.OriginalPrincipalMinor,
		UnbilledPrincipalMinor:   exposure.UnbilledPrincipalMinor,
		BilledPrincipalMinor:     exposure.BilledPrincipalMinor,
		RepaidPrincipalMinor:     exposure.RepaidPrincipalMinor,
		ReleasedPrincipalMinor:   exposure.ReleasedPrincipalMinor,
		WrittenOffPrincipalMinor: exposure.WrittenOffPrincipalMinor,
		StatusCode:               exposure.StatusCode,
		AccountTotalLimitMinor:   account.TotalLimitMinor,
		AccountAvailableMinor:    account.AvailableLimitMinor,
		AccountUnbilledMinor:     account.UnbilledPrincipalMinor,
		AccountBilledMinor:       account.BilledPrincipalMinor,
		AccountVersion:           account.Version, Reused: reused,
	}
}

func nullableCreditReference(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func creditExposureResult(result *CreditExposure, err error) (*CreditExposure, error) {
	if err == nil {
		return result, nil
	}
	if _, ok := bizerr.As(err); ok {
		return nil, err
	}
	return nil, bizerr.WrapCode(err, CodeCreditUnavailable)
}
