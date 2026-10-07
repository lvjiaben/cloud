package billing

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/asynctask"
)

const (
	creditReconciliationTaskType     = "credit.reconciliation.alert"
	maximumCreditReconciliationBatch = 100
)

type creditReconciliationServiceImpl struct {
	*serviceImpl
	tasks asynctask.Service
}

type creditReconciliationRow struct {
	CreditAccountNo         string `orm:"credit_account_no"`
	CustomerNo              string `orm:"customer_no"`
	CurrencyCode            string `orm:"currency_code"`
	StatusCode              string `orm:"status_code"`
	AccountVersion          int64  `orm:"account_version"`
	AvailableMinor          int64  `orm:"available_minor"`
	UnbilledMinor           int64  `orm:"unbilled_minor"`
	BilledMinor             int64  `orm:"billed_minor"`
	DelinquentMinor         int64  `orm:"delinquent_minor"`
	WrittenOffMinor         int64  `orm:"written_off_minor"`
	ExposureUnbilledMinor   int64  `orm:"exposure_unbilled_minor"`
	ExposureBilledMinor     int64  `orm:"exposure_billed_minor"`
	ExposureWrittenOffMinor int64  `orm:"exposure_written_off_minor"`
	CaseDelinquentMinor     int64  `orm:"case_delinquent_minor"`
	MovementVersion         int64  `orm:"movement_version"`
	MovementStatus          string `orm:"movement_status"`
	MovementAvailableMinor  int64  `orm:"movement_available_minor"`
	MovementUnbilledMinor   int64  `orm:"movement_unbilled_minor"`
	MovementBilledMinor     int64  `orm:"movement_billed_minor"`
	MovementDelinquentMinor int64  `orm:"movement_delinquent_minor"`
	MovementWrittenOffMinor int64  `orm:"movement_written_off_minor"`
}

type creditReconciliationAlertItem struct {
	CreditAccountNo  string   `json:"credit_account_no"`
	AccountVersion   int64    `json:"account_version"`
	DiscrepancyCodes []string `json:"discrepancy_codes"`
}

type creditReconciliationAlertPayload struct {
	Fingerprint string                          `json:"fingerprint"`
	Items       []creditReconciliationAlertItem `json:"items"`
}

func (s *creditReconciliationServiceImpl) ScanCreditReconciliation(
	ctx context.Context,
	afterCreditAccountNo string,
	limit int,
) (*CreditReconciliationBatch, error) {
	if afterCreditAccountNo != strings.TrimSpace(afterCreditAccountNo) ||
		(afterCreditAccountNo != "" && !validBusinessReference(afterCreditAccountNo, 40)) ||
		limit < 1 || limit > maximumCreditReconciliationBatch {
		return nil, bizerr.NewCode(CodeCreditInvalid)
	}
	var result *CreditReconciliationBatch
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		rows, loadErr := loadCreditReconciliationRows(
			ctx, s.database, afterCreditAccountNo, limit+1,
		)
		if loadErr != nil {
			return loadErr
		}
		hasMore := len(rows) > limit
		if hasMore {
			rows = rows[:limit]
		}
		items := make([]CreditReconciliationItem, 0, len(rows))
		for _, row := range rows {
			if item := projectCreditReconciliationItem(row); item != nil {
				items = append(items, *item)
			}
		}
		result = &CreditReconciliationBatch{
			Items: items, ScannedCount: len(rows), HasMore: hasMore,
		}
		if len(rows) > 0 {
			result.NextCursor = rows[len(rows)-1].CreditAccountNo
		}
		if len(items) == 0 {
			return nil
		}
		task, taskErr := s.createCreditReconciliationAlert(ctx, items)
		if taskErr != nil {
			return taskErr
		}
		result.AlertTaskNo = task.Task.TaskNo
		result.Reused = task.Reused
		return nil
	})
	return creditReconciliationResult(result, err)
}

func loadCreditReconciliationRows(
	ctx context.Context,
	database gdb.DB,
	afterCreditAccountNo string,
	limit int,
) ([]creditReconciliationRow, error) {
	rows := make([]creditReconciliationRow, 0, limit)
	if err := database.GetScan(ctx, &rows, `
WITH selected AS (
    SELECT account.credit_account_no, account.customer_no,
           account.currency_code, account.status_code, account.version,
           account.available_limit_minor, account.unbilled_principal_minor,
           account.billed_principal_minor, account.delinquent_principal_minor,
           account.written_off_principal_minor
    FROM plugin_idc_core_credit_account account
    WHERE account.tenant_id = 0 AND account.credit_account_no > ?
    ORDER BY account.credit_account_no
    LIMIT ?
), exposure AS (
    SELECT item.credit_account_no,
           COALESCE(SUM(item.unbilled_principal_minor), 0) AS unbilled_minor,
           COALESCE(SUM(item.billed_principal_minor), 0) AS billed_minor,
           COALESCE(SUM(item.written_off_principal_minor), 0) AS written_off_minor
    FROM plugin_idc_core_credit_exposure item
    JOIN selected ON selected.credit_account_no = item.credit_account_no
    GROUP BY item.credit_account_no
), delinquency AS (
    SELECT item.credit_account_no,
           COALESCE(SUM(item.principal_minor), 0) AS delinquent_minor
    FROM plugin_idc_core_credit_delinquency_case item
    JOIN selected ON selected.credit_account_no = item.credit_account_no
    WHERE item.status_code IN ('grace_period', 'frozen', 'service_action_pending')
    GROUP BY item.credit_account_no
), latest_movement AS (
    SELECT DISTINCT ON (movement.credit_account_no)
           movement.credit_account_no,
           movement.account_version_after, movement.status_after_code,
           movement.available_after_minor, movement.unbilled_after_minor,
           movement.billed_after_minor, movement.delinquent_after_minor,
           movement.written_off_after_minor
    FROM plugin_idc_core_credit_account_movement movement
    JOIN selected ON selected.credit_account_no = movement.credit_account_no
    ORDER BY movement.credit_account_no, movement.account_version_after DESC
)
SELECT selected.credit_account_no, selected.customer_no, selected.currency_code,
       selected.status_code, selected.version AS account_version,
       selected.available_limit_minor AS available_minor,
       selected.unbilled_principal_minor AS unbilled_minor,
       selected.billed_principal_minor AS billed_minor,
       selected.delinquent_principal_minor AS delinquent_minor,
       selected.written_off_principal_minor AS written_off_minor,
       COALESCE(exposure.unbilled_minor, 0) AS exposure_unbilled_minor,
       COALESCE(exposure.billed_minor, 0) AS exposure_billed_minor,
       COALESCE(exposure.written_off_minor, 0) AS exposure_written_off_minor,
       COALESCE(delinquency.delinquent_minor, 0) AS case_delinquent_minor,
       COALESCE(latest_movement.account_version_after, 0) AS movement_version,
       COALESCE(latest_movement.status_after_code, '') AS movement_status,
       COALESCE(latest_movement.available_after_minor, 0) AS movement_available_minor,
       COALESCE(latest_movement.unbilled_after_minor, 0) AS movement_unbilled_minor,
       COALESCE(latest_movement.billed_after_minor, 0) AS movement_billed_minor,
       COALESCE(latest_movement.delinquent_after_minor, 0) AS movement_delinquent_minor,
       COALESCE(latest_movement.written_off_after_minor, 0) AS movement_written_off_minor
FROM selected
LEFT JOIN exposure ON exposure.credit_account_no = selected.credit_account_no
LEFT JOIN delinquency ON delinquency.credit_account_no = selected.credit_account_no
LEFT JOIN latest_movement
  ON latest_movement.credit_account_no = selected.credit_account_no
ORDER BY selected.credit_account_no`, afterCreditAccountNo, limit); err != nil {
		return nil, gerror.Wrap(err, "load bounded credit reconciliation page")
	}
	return rows, nil
}

func projectCreditReconciliationItem(
	row creditReconciliationRow,
) *CreditReconciliationItem {
	codes := make([]string, 0, 5)
	if row.UnbilledMinor != row.ExposureUnbilledMinor {
		codes = append(codes, "exposure_unbilled_minor")
	}
	if row.BilledMinor != row.ExposureBilledMinor {
		codes = append(codes, "exposure_billed_minor")
	}
	if row.WrittenOffMinor != row.ExposureWrittenOffMinor {
		codes = append(codes, "exposure_written_off_minor")
	}
	if row.DelinquentMinor != row.CaseDelinquentMinor {
		codes = append(codes, "delinquency_principal_minor")
	}
	if row.MovementVersion != row.AccountVersion || row.MovementStatus != row.StatusCode ||
		row.MovementAvailableMinor != row.AvailableMinor ||
		row.MovementUnbilledMinor != row.UnbilledMinor ||
		row.MovementBilledMinor != row.BilledMinor ||
		row.MovementDelinquentMinor != row.DelinquentMinor ||
		row.MovementWrittenOffMinor != row.WrittenOffMinor {
		codes = append(codes, "latest_movement_projection")
	}
	if len(codes) == 0 {
		return nil
	}
	return &CreditReconciliationItem{
		CreditAccountNo: row.CreditAccountNo, CustomerNo: row.CustomerNo,
		CurrencyCode: row.CurrencyCode, AccountVersion: row.AccountVersion,
		DiscrepancyCodes: codes, ProjectedUnbilledMinor: row.UnbilledMinor,
		ExposureUnbilledMinor:        row.ExposureUnbilledMinor,
		ProjectedBilledMinor:         row.BilledMinor,
		ExposureBilledMinor:          row.ExposureBilledMinor,
		ProjectedDelinquentMinor:     row.DelinquentMinor,
		CaseDelinquentMinor:          row.CaseDelinquentMinor,
		ProjectedWrittenOffMinor:     row.WrittenOffMinor,
		ExposureWrittenOffMinor:      row.ExposureWrittenOffMinor,
		LatestMovementAccountVersion: row.MovementVersion,
	}
}

func (s *creditReconciliationServiceImpl) createCreditReconciliationAlert(
	ctx context.Context,
	items []CreditReconciliationItem,
) (*asynctask.CreateResult, error) {
	fingerprint, err := contentDigest(items)
	if err != nil {
		return nil, err
	}
	payloadItems := make([]creditReconciliationAlertItem, 0, len(items))
	for _, item := range items {
		payloadItems = append(payloadItems, creditReconciliationAlertItem{
			CreditAccountNo: item.CreditAccountNo, AccountVersion: item.AccountVersion,
			DiscrepancyCodes: item.DiscrepancyCodes,
		})
	}
	payload, err := json.Marshal(creditReconciliationAlertPayload{
		Fingerprint: fingerprint, Items: payloadItems,
	})
	if err != nil {
		return nil, gerror.Wrap(err, "encode credit reconciliation alert")
	}
	subjectNo := deterministicCreditNo("crb_", "reconciliation", fingerprint)
	return s.tasks.CreateInDatabase(ctx, s.database, asynctask.CreateInput{
		TaskType:    creditReconciliationTaskType,
		SubjectType: "credit_reconciliation_batch", SubjectNo: subjectNo,
		IdempotencyKey: fingerprint, Priority: 100, Payload: payload,
	})
}

func creditReconciliationResult(
	result *CreditReconciliationBatch,
	err error,
) (*CreditReconciliationBatch, error) {
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
