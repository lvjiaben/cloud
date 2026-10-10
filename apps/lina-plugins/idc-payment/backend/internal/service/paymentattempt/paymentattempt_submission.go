// This file retains uncertain provider submission outcomes without releasing
// held funds, and schedules only explicitly verified query capabilities.
package paymentattempt

import (
	"context"
	"encoding/json"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-payment/backend/cap/paymentcap"
)

// MarkSubmissionUnknown records a possibly accepted provider request. A
// verified success already received through a callback always takes precedence.
// Query work is created only when the channel has verified query capability;
// otherwise the payment awaits a callback or manual reconciliation.
func (s *Store) MarkSubmissionUnknown(ctx context.Context, attemptNo string) error {
	return s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		row, err := tx.GetOne(`SELECT a.status_code,c.capability_snapshot FROM plugin_idc_payment_attempt a JOIN plugin_idc_payment_channel c ON c.channel_no=a.channel_no WHERE a.attempt_no=? FOR UPDATE OF a`, attemptNo)
		if err != nil {
			return err
		}
		if row.IsEmpty() {
			return gerror.New("payment attempt is not visible")
		}
		if row["status_code"].String() != "created" && row["status_code"].String() != "submitted" {
			return nil
		}
		if _, err = tx.Exec(`UPDATE plugin_idc_payment_attempt SET status_code='manual_review',result_unknown=TRUE,updated_at=CURRENT_TIMESTAMP,version=version+1 WHERE attempt_no=?`, attemptNo); err != nil {
			return err
		}
		var operations map[string]bool
		if err = json.Unmarshal(row["capability_snapshot"].Bytes(), &operations); err != nil {
			return gerror.Wrap(err, "payment channel capability snapshot is invalid")
		}
		if !operations[string(paymentcap.OperationQuery)] {
			return nil
		}
		_, err = tx.Exec(`INSERT INTO plugin_idc_payment_work_item(work_no,work_type,subject_no,evidence_digest) VALUES(?,'query_payment',?,repeat('0',64)) ON CONFLICT(work_type,subject_no) DO NOTHING`, stableNo("pwk", "query_payment", attemptNo), attemptNo)
		return err
	})
}
