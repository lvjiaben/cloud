package saleschannel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
)

// MatureDueCommissions moves due, unreversed accruals from the pending bucket
// to the available bucket. Every move writes one immutable maturity record and
// one account-ledger entry in the same transaction, so a retry cannot pay the
// same commission twice.
func (s *serviceImpl) MatureDueCommissions(ctx context.Context, now time.Time, limit int) (int, error) {
	if s.database == nil || ctx == nil || now.IsZero() || limit < 1 || limit > 100 {
		return 0, gerror.New("commission maturity request is invalid")
	}
	processed := 0
	err := s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		rows, err := tx.GetAll(`
SELECT commission_no,beneficiary_type_code,beneficiary_reference,currency_code,
       pending_amount_minor,matures_at
  FROM plugin_idc_core_commission_accrual
 WHERE tenant_id=0 AND pending_amount_minor>0
   AND status_code IN ('pending_maturity','partially_reversed')
   AND matures_at<=?
 ORDER BY matures_at,id
 LIMIT ? FOR UPDATE`, now, limit)
		if err != nil {
			return err
		}
		for _, row := range rows {
			amount := row["pending_amount_minor"].Int64()
			if amount <= 0 {
				continue
			}
			commissionNo := row["commission_no"].String()
			currency := row["currency_code"].String()
			beneficiaryType := row["beneficiary_type_code"].String()
			beneficiary := row["beneficiary_reference"].String()
			accountRows, queryErr := tx.GetAll(`
SELECT account_no,credited_amount_minor,pending_amount_minor,available_amount_minor,
       frozen_amount_minor,paid_amount_minor,reversed_amount_minor,version
  FROM plugin_idc_core_sales_commission_account
 WHERE tenant_id=0 AND beneficiary_type_code=? AND beneficiary_reference=?
   AND currency_code=? AND status_code='active' FOR UPDATE`, beneficiaryType, beneficiary, currency)
			if queryErr != nil {
				return queryErr
			}
			if len(accountRows) != 1 {
				return gerror.Newf("commission account is missing or ambiguous: %s", commissionNo)
			}
			account := accountRows[0]
			availablePending := account["pending_amount_minor"].Int64()
			if availablePending < amount {
				return gerror.Newf("commission account pending balance is insufficient: %s", commissionNo)
			}
			attemptRows, queryErr := tx.GetAll(`SELECT COALESCE(MAX(attempt_no),0) AS attempt_no FROM plugin_idc_core_sales_commission_maturity WHERE commission_no=?`, commissionNo)
			if queryErr != nil {
				return queryErr
			}
			attempt := 1
			if len(attemptRows) > 0 {
				attempt = attemptRows[0]["attempt_no"].Int() + 1
			}
			evidence := sha256.Sum256([]byte(fmt.Sprintf("maturity:%s:%d:%d", commissionNo, attempt, amount)))
			digest := hex.EncodeToString(evidence[:])
			maturityNo := "MAT_" + digest[:32]
			if _, insertErr := tx.Exec(`INSERT INTO plugin_idc_core_sales_commission_maturity
 (tenant_id,maturity_no,commission_no,attempt_no,account_no,currency_code,beneficiary_type_code,beneficiary_reference,amount_minor,status_code,reason_code,evidence_digest)
 VALUES (0,?,?,?,?,?,?,?,?,?,'matured','maturity_due',?)`, maturityNo, commissionNo, attempt, account["account_no"].String(), currency, beneficiaryType, beneficiary, amount, digest); insertErr != nil {
				return insertErr
			}
			beforeVersion := account["version"].Int64()
			if _, insertErr := tx.Exec(`INSERT INTO plugin_idc_core_sales_commission_account_entry
 (tenant_id,entry_no,account_no,currency_code,entry_sequence,entry_type_code,source_type_code,source_reference,source_digest,
  delta_credited_minor,delta_pending_minor,delta_available_minor,delta_frozen_minor,delta_paid_minor,delta_reversed_minor,
  after_credited_minor,after_pending_minor,after_available_minor,after_frozen_minor,after_paid_minor,after_reversed_minor)
 VALUES (0,?,?,?,?,?,?,?,?,0,?,?,0,0,0,?,?,?,?,?,?)`, "CENT_"+digest[:33], account["account_no"].String(), currency, beforeVersion+1,
				"maturity", "commission_maturity", maturityNo, digest, -amount, amount,
				account["credited_amount_minor"].Int64(), availablePending-amount, account["available_amount_minor"].Int64()+amount,
				account["frozen_amount_minor"].Int64(), account["paid_amount_minor"].Int64(), account["reversed_amount_minor"].Int64()); insertErr != nil {
				return insertErr
			}
			if _, updateErr := tx.Exec(`UPDATE plugin_idc_core_commission_accrual
 SET pending_amount_minor=pending_amount_minor-?, matured_amount_minor=matured_amount_minor+?, status_code='matured', updated_at=CURRENT_TIMESTAMP
 WHERE commission_no=? AND pending_amount_minor>=?`, amount, amount, commissionNo, amount); updateErr != nil {
				return updateErr
			}
			processed++
		}
		return nil
	})
	if err != nil {
		return processed, gerror.Wrap(err, "mature due commissions")
	}
	return processed, nil
}
