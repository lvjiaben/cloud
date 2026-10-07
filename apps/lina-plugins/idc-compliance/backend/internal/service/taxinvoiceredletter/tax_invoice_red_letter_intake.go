// This file consumes completed refunds and advances a durable source cursor only after commit.

package taxinvoiceredletter

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/taxinvoicecap"

	"lina-plugin-idc-compliance/backend/internal/dao"
	"lina-plugin-idc-compliance/backend/internal/model/do"
)

const postedRefundFeedCode = "posted_refund"

// IntakeWorker consumes posted refunds in source order and advances its cursor
// only after the corresponding task or no-correction decision commits.
type IntakeWorker interface {
	// Run consumes at most limit posted refunds and leaves the durable cursor
	// unchanged when any refund cannot be recorded safely.
	Run(context.Context, int) (int, error)
}

type intakeWorker struct {
	database gdb.DB
	feed     taxinvoicecap.PostedRefundFeed
	recorder Service
}

// NewIntakeWorker constructs the durable primary-node refund consumer.
func NewIntakeWorker(
	database gdb.DB,
	feed taxinvoicecap.PostedRefundFeed,
	recorder Service,
) (IntakeWorker, error) {
	if nilDependency(database) || nilDependency(feed) || nilDependency(recorder) {
		return nil, gerror.New("red-letter intake dependencies are invalid")
	}
	return &intakeWorker{database: database, feed: feed, recorder: recorder}, nil
}

func (w *intakeWorker) Run(ctx context.Context, limit int) (int, error) {
	if ctx == nil || ctx.Err() != nil || limit < 1 || limit > taxinvoicecap.MaximumRefundOperations {
		return 0, gerror.New("red-letter intake request is invalid")
	}
	var (
		cursorDAO     = dao.RedLetterIntakeCursor
		cursorColumns = cursorDAO.Columns()
		cursorKey     = do.RedLetterIntakeCursor{
			TenantId: idccap.DefaultTenantID,
			FeedCode: postedRefundFeedCode,
		}
	)
	if _, err := w.database.Model(cursorDAO.Table()).Ctx(ctx).
		Data(do.RedLetterIntakeCursor{
			TenantId: idccap.DefaultTenantID, FeedCode: postedRefundFeedCode,
			SourceCursor: "", Version: 1,
		}).OnConflict(cursorColumns.TenantId, cursorColumns.FeedCode).InsertIgnore(); err != nil {
		return 0, gerror.Wrap(err, "initialize red-letter intake cursor")
	}
	cursorValue, err := w.database.Model(cursorDAO.Table()).Ctx(ctx).
		Fields(cursorColumns.SourceCursor).
		Where(cursorKey).
		Value()
	if err != nil {
		return 0, gerror.Wrap(err, "load red-letter intake cursor")
	}
	cursor := cursorValue.String()
	rows, err := w.feed.ListPostedRefundOperations(ctx, cursor, limit)
	if err != nil {
		return 0, gerror.Wrap(err, "list posted refunds for red-letter intake")
	}
	processed := 0
	for _, row := range rows {
		if !businessNoPattern.MatchString(row.RefundOperationNo) ||
			row.Cursor == "" || len(row.Cursor) > 64 || row.Cursor == cursor {
			return processed, gerror.New("posted refund feed returned invalid data")
		}
		digest := sha256.Sum256([]byte("red-letter-intake\x00" + row.RefundOperationNo))
		if _, err = w.recorder.RecordPostedRefund(
			ctx, row.RefundOperationNo, hex.EncodeToString(digest[:]),
		); err != nil {
			return processed, err
		}
		result, updateErr := w.database.Model(cursorDAO.Table()).Ctx(ctx).
			Where(do.RedLetterIntakeCursor{
				TenantId: idccap.DefaultTenantID, FeedCode: postedRefundFeedCode,
				SourceCursor: cursor,
			}).
			Data(do.RedLetterIntakeCursor{
				SourceCursor: row.Cursor,
				Version:      gdb.Raw(cursorColumns.Version + " + 1"),
			}).
			Update()
		if updateErr != nil {
			return processed, gerror.Wrap(updateErr, "advance red-letter intake cursor")
		}
		affected, affectedErr := result.RowsAffected()
		if affectedErr != nil || affected != 1 {
			return processed, gerror.New("red-letter intake cursor changed concurrently")
		}
		cursor = row.Cursor
		processed++
	}
	return processed, nil
}
