// This file releases a newly protected write without consuming its retry budget.

package resourcetask

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-resource/backend/internal/dao"
	"lina-plugin-idc-resource/backend/internal/model/do"
)

// DeferWriteForProtection releases the exact live lease and restores its
// attempt count. It is valid only before the Provider call boundary.
func (s *Store) DeferWriteForProtection(
	ctx context.Context,
	lease LeaseRef,
	nextAt time.Time,
) error {
	if s == nil || s.database == nil || ctx == nil || ctx.Err() != nil {
		return gerror.New("resource task store is unavailable")
	}
	if err := validateLeaseRef(lease); err != nil {
		return err
	}
	return s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		now, err := resourceDatabaseTime(tx)
		if err != nil {
			return err
		}
		next := nextAt.UTC()
		if !next.After(now) || next.After(now.Add(maximumTransitionScheduleDelay)) {
			return gerror.New("resource task protection schedule is invalid")
		}
		row, err := loadLiveSubmittingTask(ctx, tx, lease, now)
		if err != nil {
			return err
		}
		if row == nil {
			return gerror.New("resource task lease is unavailable")
		}
		columns := dao.Task.Columns()
		result, err := tx.Model(dao.Task.Table()).Safe().Ctx(ctx).
			Where(do.Task{
				TenantId: idccap.DefaultTenantID, Id: row.Id, TaskNo: lease.TaskNo,
				StatusCode: StatusSubmitting, LockedBy: lease.Token,
				AttemptCount: lease.AttemptNo,
			}).
			Data(do.Task{
				StatusCode: StatusRetryWait, LockedBy: "", AvailableAt: &next,
				AttemptCount: gdb.Raw(columns.AttemptCount + " - 1"),
				FailureCode:  "", FailureSummary: "",
				Version: gdb.Raw(columns.Version + " + 1"),
			}).Update()
		if err != nil {
			return gerror.Wrap(err, "defer protected resource task")
		}
		if err = requireAffected(result, 1, "defer protected resource task"); err != nil {
			return err
		}
		return clearTaskLeaseExpiries(ctx, tx, []int64{row.Id})
	})
}
