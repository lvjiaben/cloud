// This file owns bounded administrative directory pages using generated models.
// Count and safe row projections share one repeatable-read, read-only transaction.

package exceptionfund

import (
	"context"
	"database/sql"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-payment/backend/internal/dao"
	"lina-plugin-idc-payment/backend/internal/model/do"
)

// directoryStatus names existing pending/disputed states without changing their meaning.
type directoryStatus string

const (
	directoryPending  directoryStatus = "pending"
	directoryDisputed directoryStatus = "disputed"
)

// List returns an exact pending/disputed directory page without changing funds.
// Invalid bounds or storage failures return localized errors without partial rows.
func (s *Service) List(ctx context.Context, page, pageSize int) (*Page, error) {
	if page < 1 || page > 1_000_000 || pageSize < 1 || pageSize > 100 {
		return nil, bizerr.NewCode(CodePageInvalid)
	}
	var (
		total int
		rows  gdb.Result
	)
	err := s.database.TransactionWithOptions(ctx, gdb.TxOptions{Propagation: gdb.PropagationRequired, Isolation: sql.LevelRepeatableRead, ReadOnly: true}, func(ctx context.Context, _ gdb.TX) error {
		cols := dao.ExceptionFund.Columns()
		model := s.database.Model(do.ExceptionFund{}).Ctx(ctx).As("f").WhereIn("f."+cols.StatusCode, []directoryStatus{directoryPending, directoryDisputed})
		var err error
		total, err = model.Clone().Count()
		if err != nil {
			return err
		}
		// Funds retain their historical channel metadata after the channel is
		// soft-deleted. Unscoped preserves the original projection, not access.
		rows, err = model.Unscoped().InnerJoin(dao.Attempt.Table(), "a", "a.attempt_no=f.attempt_no").
			InnerJoin(dao.Channel.Table(), "c", "c.channel_no=f.channel_no").
			LeftJoin(dao.ExceptionFundAction.Table(), "x", "x.exception_no=f.exception_no").
			Fields("f.exception_no,f.attempt_no,a.order_no,f.channel_no,c.provider_code,f.currency_code,f.amount_minor,f.reason_code,f.status_code,f.evidence_digest,f.created_at,x.action_no,x.action_code,x.status_code AS action_status,x.target_order_no,x.proof_reference,x.version").
			OrderAsc("f."+cols.CreatedAt).OrderAsc("f."+cols.Id).Page(page, pageSize).All()
		return err
	})
	if err != nil {
		return nil, bizerr.WrapCode(err, CodePageUnavailable)
	}
	result := make([]*Item, 0, len(rows))
	for _, row := range rows {
		result = append(result, mapItem(row))
	}
	return &Page{Items: result, Total: total}, nil
}
