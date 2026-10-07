// This file owns bounded administrative directory pages using generated models.
// Count and safe row projections share one repeatable-read, read-only transaction.

package reconciliation

import (
	"context"
	"database/sql"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-payment/backend/internal/dao"
	"lina-plugin-idc-payment/backend/internal/model/do"
)

// directoryStatus names existing values used by the administrative projection.
type directoryStatus string

const (
	directoryDifferenceNone        directoryStatus = "none"
	directoryCapabilityUnavailable directoryStatus = "unavailable"
	directoryHealthNeverRun        directoryStatus = "never_run"
)

// ListHealth reads one channel health page and total from the same read-only snapshot.
// Invalid bounds or unavailable storage fail without disclosing partial data.
func (s *Service) ListHealth(ctx context.Context, page, pageSize int) (*HealthPage, error) {
	if !validNumericPage(page, pageSize) {
		return nil, bizerr.NewCode(CodePageInvalid)
	}
	var (
		total int
		rows  gdb.Result
	)
	err := s.database.TransactionWithOptions(ctx, gdb.TxOptions{Propagation: gdb.PropagationRequired, Isolation: sql.LevelRepeatableRead, ReadOnly: true}, func(ctx context.Context, _ gdb.TX) error {
		model := s.database.Model(do.Channel{}).Ctx(ctx).As("c").WherePrefix("c", do.Channel{TenantId: 0})
		var err error
		total, err = model.Clone().Count()
		if err != nil {
			return err
		}
		cols := dao.Channel.Columns()
		rows, err = model.LeftJoin(dao.ReconciliationChannelHealth.Table(), "h", "h.channel_no=c.channel_no").
			Fields("c.channel_no,c.display_name,c.provider_code,h.capability_code,h.health_code,h.latest_batch_no,h.latest_business_date,h.unresolved_count,h.consecutive_failure_count,h.result_unknown,h.failure_code").
			OrderAsc("c."+cols.DisplayName).OrderAsc("c."+cols.ChannelNo).Page(page, pageSize).All()
		return err
	})
	if err != nil {
		return nil, bizerr.WrapCode(err, CodePageUnavailable)
	}
	items := make([]ChannelHealth, 0, len(rows))
	for _, row := range rows {
		item := ChannelHealth{ChannelNo: row["channel_no"].String(), DisplayName: row["display_name"].String(), ProviderCode: row["provider_code"].String(), CapabilityCode: row["capability_code"].String(), HealthCode: row["health_code"].String(), LatestBatchNo: row["latest_batch_no"].String(), UnresolvedCount: row["unresolved_count"].Int(), ConsecutiveFailureCount: row["consecutive_failure_count"].Int(), ResultUnknown: row["result_unknown"].Bool(), FailureCode: row["failure_code"].String()}
		if row["capability_code"].IsNil() {
			item.CapabilityCode = string(directoryCapabilityUnavailable)
		}
		if row["health_code"].IsNil() {
			item.HealthCode = string(directoryHealthNeverRun)
		}
		if !row["latest_business_date"].IsNil() {
			value := row["latest_business_date"].Time()
			item.LatestBusinessDate = &value
		}
		items = append(items, item)
	}
	return &HealthPage{Items: items, Total: total}, nil
}

// ListBatches preserves the optional exact channel scope in both count and rows.
// Empty channelNo keeps the merchant-wide scope; invalid bounds fail closed.
func (s *Service) ListBatches(ctx context.Context, channelNo string, page, pageSize int) (*BatchPage, error) {
	if !validNumericPage(page, pageSize) {
		return nil, bizerr.NewCode(CodePageInvalid)
	}
	var (
		total int
		rows  gdb.Result
	)
	err := s.database.TransactionWithOptions(ctx, gdb.TxOptions{Propagation: gdb.PropagationRequired, Isolation: sql.LevelRepeatableRead, ReadOnly: true}, func(ctx context.Context, _ gdb.TX) error {
		model := s.database.Model(do.ReconciliationBatch{}).Ctx(ctx)
		if channelNo != "" {
			model = model.Where(do.ReconciliationBatch{ChannelNo: channelNo})
		}
		var err error
		total, err = model.Clone().Count()
		if err != nil {
			return err
		}
		cols := dao.ReconciliationBatch.Columns()
		rows, err = model.Fields(cols.BatchNo, cols.ChannelNo, cols.BusinessDate, cols.CapabilityCode, cols.StatusCode, cols.TotalCount, cols.DifferenceCount, cols.PaymentCount, cols.RefundCount, cols.ResultUnknown, cols.FailureCode).
			OrderDesc(cols.BusinessDate).OrderDesc(cols.Id).Page(page, pageSize).All()
		return err
	})
	if err != nil {
		return nil, bizerr.WrapCode(err, CodePageUnavailable)
	}
	result := &BatchPage{Items: make([]BatchResult, 0, len(rows)), Total: total}
	for _, row := range rows {
		item := BatchResult{BatchNo: row["batch_no"].String(), ChannelNo: row["channel_no"].String(), CapabilityCode: row["capability_code"].String(), StatusCode: row["status_code"].String(), BusinessDate: row["business_date"].Time(), TotalCount: row["total_count"].Int(), DifferenceCount: row["difference_count"].Int(), PaymentCount: row["payment_count"].Int(), RefundCount: row["refund_count"].Int(), ResultUnknown: row["result_unknown"].Bool(), FailureCode: row["failure_code"].String()}
		result.Items = append(result.Items, item)
	}
	return result, nil
}

// ListDifferences pages only non-matching evidence owned by the requested batch.
// Unknown batches are empty; invalid bounds and storage failures are localized errors.
func (s *Service) ListDifferences(ctx context.Context, batchNo string, page, pageSize int) (*DifferencePage, error) {
	if batchNo == "" || !validNumericPage(page, pageSize) {
		return nil, bizerr.NewCode(CodePageInvalid)
	}
	var (
		total int
		rows  gdb.Result
	)
	err := s.database.TransactionWithOptions(ctx, gdb.TxOptions{Propagation: gdb.PropagationRequired, Isolation: sql.LevelRepeatableRead, ReadOnly: true}, func(ctx context.Context, _ gdb.TX) error {
		cols := dao.ReconciliationLine.Columns()
		model := s.database.Model(do.ReconciliationLine{}).Ctx(ctx).As("l").WherePrefix("l", do.ReconciliationLine{BatchNo: batchNo}).WhereNot("l."+cols.DifferenceCode, directoryDifferenceNone)
		var err error
		total, err = model.Clone().Count()
		if err != nil {
			return err
		}
		rows, err = model.LeftJoin(dao.ReconciliationSupplement.Table(), "s", "s.line_no=l.line_no").
			Fields("l.line_no,l.batch_no,l.record_type_code,l.attempt_no,l.refund_attempt_no,l.external_reference,l.difference_code,l.local_amount_minor,l.channel_amount_minor,l.currency_code,l.local_status_code,l.channel_status_code,l.resolution_code,l.version,s.request_no AS supplement_request_no,s.status_code AS supplement_status_code,s.submitter_user_id AS supplement_submitter_user_id,s.version AS supplement_version").
			OrderAsc("l."+cols.Id).Page(page, pageSize).All()
		return err
	})
	if err != nil {
		return nil, bizerr.WrapCode(err, CodePageUnavailable)
	}
	items := make([]Difference, 0, len(rows))
	for _, row := range rows {
		items = append(items, Difference{LineNo: row["line_no"].String(), BatchNo: row["batch_no"].String(), RecordType: row["record_type_code"].String(), AttemptNo: row["attempt_no"].String(), RefundAttemptNo: row["refund_attempt_no"].String(), ExternalReference: row["external_reference"].String(), DifferenceCode: row["difference_code"].String(), LocalAmountMinor: row["local_amount_minor"].Int64(), ChannelAmountMinor: row["channel_amount_minor"].Int64(), CurrencyCode: row["currency_code"].String(), LocalStatusCode: row["local_status_code"].String(), ChannelStatusCode: row["channel_status_code"].String(), ResolutionCode: row["resolution_code"].String(), SupplementRequestNo: row["supplement_request_no"].String(), SupplementStatusCode: row["supplement_status_code"].String(), SupplementSubmitterUserID: row["supplement_submitter_user_id"].Int(), Version: row["version"].Int64(), SupplementVersion: row["supplement_version"].Int64()})
	}
	return &DifferencePage{Items: items, Total: total}, nil
}

func validNumericPage(page, pageSize int) bool {
	return page >= 1 && page <= 1_000_000 && pageSize >= 1 && pageSize <= 100
}
