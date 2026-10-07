// This file discovers due merchant-timezone credit periods with one bounded
// query and delegates each selected account to the idempotent statement owner.

package billing

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
)

const maximumCreditStatementBatch = 64

type creditStatementCandidate struct {
	CreditAccountNo string    `orm:"credit_account_no"`
	PeriodStartedAt time.Time `orm:"period_started_at"`
	PeriodEndedAt   time.Time `orm:"period_ended_at"`
}

// RunCreditStatementBatch processes at most limit due accounts. Candidate
// discovery is one set query; the bounded loop performs independent business
// transactions so one damaged account cannot suppress the remaining batch.
func (s *serviceImpl) RunCreditStatementBatch(
	ctx context.Context,
	limit int,
) (*CreditStatementBatch, error) {
	if s == nil || s.database == nil || ctx == nil || ctx.Err() != nil ||
		limit <= 0 || limit > maximumCreditStatementBatch {
		return nil, bizerr.NewCode(CodeCreditInvalid)
	}
	timezone, err := loadCreditMerchantTimezone(ctx, s.database)
	if err != nil {
		return nil, creditStatementBatchError(err)
	}
	candidates := make([]creditStatementCandidate, 0, limit)
	if err = s.database.GetScan(ctx, &candidates, `
WITH account_period AS (
    SELECT account.id, account.credit_account_no, terms.billing_day,
           GREATEST(
               terms.effective_from,
               COALESCE(MAX(statement.period_ended_at), terms.effective_from)
           ) AS last_period_end,
           MIN(exposure.created_at) AS oldest_unbilled_at
    FROM plugin_idc_core_credit_account account
    INNER JOIN plugin_idc_core_customer_subject customer
        ON customer.tenant_id = account.tenant_id
       AND customer.customer_no = account.customer_no
       AND customer.status_code = 'active'
    INNER JOIN plugin_idc_core_credit_account_terms terms
        ON terms.tenant_id = account.tenant_id
       AND terms.credit_account_no = account.credit_account_no
       AND terms.terms_no = account.current_terms_no
       AND terms.terms_version = account.current_terms_version
    INNER JOIN plugin_idc_core_credit_exposure exposure
        ON exposure.tenant_id = account.tenant_id
       AND exposure.credit_account_no = account.credit_account_no
       AND exposure.unbilled_principal_minor > 0
    LEFT JOIN plugin_idc_core_credit_statement statement
        ON statement.tenant_id = account.tenant_id
       AND statement.credit_account_no = account.credit_account_no
    WHERE account.tenant_id = 0
      AND account.status_code IN ('active', 'frozen', 'expired')
    GROUP BY account.id, account.credit_account_no, terms.billing_day,
             terms.effective_from
), due_period AS (
    SELECT id, credit_account_no, billing_day, last_period_end, oldest_unbilled_at,
           GREATEST(last_period_end, oldest_unbilled_at) AT TIME ZONE ? AS local_anchor
    FROM account_period
), billing_cutoff AS (
    SELECT id, credit_account_no, billing_day, last_period_end, local_anchor,
           date_trunc('month', local_anchor)
             + (billing_day - 1) * INTERVAL '1 day' AS current_month_cutoff
    FROM due_period
), due_account AS (
    SELECT id, credit_account_no, last_period_end,
           CASE WHEN current_month_cutoff > local_anchor
                THEN current_month_cutoff
                ELSE current_month_cutoff + INTERVAL '1 month'
           END AS local_period_end
    FROM billing_cutoff
)
SELECT credit_account_no,
       GREATEST(
           last_period_end,
           (local_period_end - INTERVAL '1 month') AT TIME ZONE ?
       ) AS period_started_at,
       local_period_end AT TIME ZONE ? AS period_ended_at
FROM due_account
WHERE local_period_end AT TIME ZONE ? <= CURRENT_TIMESTAMP
ORDER BY local_period_end ASC, id ASC
LIMIT ?`, timezone, timezone, timezone, timezone, limit); err != nil {
		return nil, creditStatementBatchError(gerror.Wrap(err, "list due credit statements"))
	}
	report := &CreditStatementBatch{Selected: len(candidates)}
	for _, candidate := range candidates {
		if ctx.Err() != nil {
			return report, creditStatementBatchError(ctx.Err())
		}
		statement, issueErr := s.IssueCreditStatement(ctx, CreditStatementIssueInput{
			CreditAccountNo: candidate.CreditAccountNo,
			PeriodStartedAt: candidate.PeriodStartedAt,
			PeriodEndedAt:   candidate.PeriodEndedAt,
			IdempotencyKey: deterministicCreditNo(
				"cis_", candidate.CreditAccountNo,
				candidate.PeriodEndedAt.UTC().Format(time.RFC3339Nano),
			),
		})
		if issueErr != nil || statement == nil {
			report.Failures++
			continue
		}
		if statement.Reused {
			report.Reused++
			continue
		}
		report.Issued++
	}
	if report.Failures != 0 {
		return report, bizerr.NewCode(CodeCreditUnavailable)
	}
	return report, nil
}

func creditStatementBatchError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := bizerr.As(err); ok {
		return err
	}
	return bizerr.WrapCode(err, CodeCreditUnavailable)
}
