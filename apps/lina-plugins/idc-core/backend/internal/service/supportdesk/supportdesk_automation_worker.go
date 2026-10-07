package supportdesk

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
)

// AutomationWorker applies database-authoritative support rules in bounded,
// skip-locked batches. It never changes tickets that received a customer reply
// after resolution because those tickets no longer remain in the resolved stage.
type AutomationWorker interface {
	AutoCloseResolved(context.Context, int) (int, error)
}

type automationWorker struct {
	database gdb.DB
	random   io.Reader
}

func NewAutomationWorker(database gdb.DB) (AutomationWorker, error) {
	if database == nil {
		return nil, gerror.New("support automation worker requires database")
	}
	return &automationWorker{database: database, random: rand.Reader}, nil
}

func (w *automationWorker) AutoCloseResolved(ctx context.Context, limit int) (int, error) {
	if ctx == nil || ctx.Err() != nil || limit < 1 || limit > 500 {
		return 0, gerror.New("support automation worker input is invalid")
	}
	closed := 0
	err := w.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		now, err := databaseNow(ctx, w.database)
		if err != nil {
			return err
		}
		var rows []struct {
			TicketNo     string `orm:"ticket_no"`
			CurrentRound int    `orm:"current_round"`
			Version      int64  `orm:"version"`
			RuleVersion  int64  `orm:"rule_version"`
		}
		if err = w.database.GetScan(ctx, &rows, `SELECT t.ticket_no,t.current_round,t.version,p.version AS rule_version FROM plugin_idc_core_support_ticket t JOIN plugin_idc_core_support_automation_policy p ON p.tenant_id=0 AND p.policy_no='support-default' WHERE t.tenant_id=0 AND t.base_stage='resolved' AND t.resolved_at IS NOT NULL AND t.resolved_at+(p.auto_close_days*INTERVAL '1 day')<=? AND (t.customer_last_replied_at IS NULL OR t.customer_last_replied_at<=t.resolved_at) ORDER BY t.resolved_at,t.id LIMIT ? FOR UPDATE OF t SKIP LOCKED`, now, limit); err != nil {
			return err
		}
		for _, row := range rows {
			historyNo, noErr := w.newHistoryNo()
			if noErr != nil {
				return noErr
			}
			result, updateErr := w.database.Exec(ctx, `UPDATE plugin_idc_core_support_ticket SET base_stage='closed',closed_at=?,closed_reason_code='auto_close',closed_by_type='automation',sla_risk_code=CASE WHEN sla_risk_code='at_risk' THEN 'none' ELSE sla_risk_code END,last_activity_at=?,version=version+1,updated_by=0,updated_at=? WHERE tenant_id=0 AND ticket_no=? AND base_stage='resolved' AND version=?`, now, now, now, row.TicketNo, row.Version)
			if updateErr != nil {
				return updateErr
			}
			changed, updateErr := result.RowsAffected()
			if updateErr != nil {
				return updateErr
			}
			if changed != 1 {
				return gerror.New("support auto-close ticket changed while locked")
			}
			if _, updateErr = w.database.Exec(ctx, `UPDATE plugin_idc_core_support_ticket_sla_round SET stopped_at=COALESCE(stopped_at,?) WHERE tenant_id=0 AND ticket_no=? AND round_no=?`, now, row.TicketNo, row.CurrentRound); updateErr != nil {
				return updateErr
			}
			if _, updateErr = w.database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket_status_history (tenant_id,history_no,ticket_no,from_base_stage,to_base_stage,source_code,actor_user_id,reason_code,rule_version,occurred_at) VALUES (0,? ,?,'resolved','closed','automation',0,'auto_close',?,?)`, historyNo, row.TicketNo, row.RuleVersion, now); updateErr != nil {
				return updateErr
			}
			closed++
		}
		return nil
	})
	if err != nil {
		return 0, gerror.Wrap(err, "auto-close resolved support tickets")
	}
	return closed, nil
}

func (w *automationWorker) newHistoryNo() (string, error) {
	var value [16]byte
	if _, err := io.ReadFull(w.random, value[:]); err != nil {
		return "", gerror.Wrap(err, "generate support automation history number")
	}
	return "TSH_" + hex.EncodeToString(value[:]), nil
}
