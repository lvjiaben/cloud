package supportdesk

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
)

const (
	SLARiskNone     = "none"
	SLARiskAtRisk   = "at_risk"
	SLARiskBreached = "breached"
)

// SLAWorker refreshes the persisted current-round risk projection. It does not
// mark reminder nodes as sent: that requires the separately fenced notification
// event boundary and current staff-recipient authorization proof.
type SLAWorker interface {
	RefreshRisk(context.Context, int) (int, error)
}

type slaWorker struct{ database gdb.DB }

type slaRiskRow struct {
	TicketNo                string     `orm:"ticket_no"`
	RoundNo                 int        `orm:"round_no"`
	Risk                    string     `orm:"sla_risk_code"`
	NearDueMinutes          []int      `orm:"near_due_minutes"`
	FirstResponseDueAt      *time.Time `orm:"first_response_due_at"`
	ResolutionDueAt         *time.Time `orm:"resolution_due_at"`
	FirstRespondedAt        *time.Time `orm:"first_responded_at"`
	ResolvedAt              *time.Time `orm:"resolved_at"`
	StoppedAt               *time.Time `orm:"stopped_at"`
	FirstResponseBreached   bool       `orm:"first_response_breached"`
	ResolutionBreached      bool       `orm:"resolution_breached"`
	ResolutionTimerIsPaused bool       `orm:"resolution_timer_is_paused"`
}

func NewSLAWorker(database gdb.DB) (SLAWorker, error) {
	if database == nil {
		return nil, gerror.New("support SLA worker requires database")
	}
	return &slaWorker{database: database}, nil
}

func (w *slaWorker) RefreshRisk(ctx context.Context, limit int) (int, error) {
	if ctx == nil || ctx.Err() != nil {
		return 0, gerror.New("support SLA worker context is unavailable")
	}
	if limit < 1 || limit > 500 {
		return 0, gerror.New("support SLA worker batch limit is invalid")
	}
	changed := 0
	err := w.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		now, err := databaseNow(ctx, w.database)
		if err != nil {
			return err
		}
		var rows []slaRiskRow
		if err = w.database.GetScan(ctx, &rows, `SELECT t.ticket_no,r.round_no,t.sla_risk_code,r.near_due_minutes,r.first_response_due_at,r.resolution_due_at,r.first_responded_at,r.resolved_at,r.stopped_at,r.first_response_breached,r.resolution_breached,EXISTS (SELECT 1 FROM plugin_idc_core_support_ticket_sla_pause p WHERE p.tenant_id=0 AND p.ticket_no=t.ticket_no AND p.round_no=t.current_round AND p.ended_at IS NULL) AS resolution_timer_is_paused FROM plugin_idc_core_support_ticket t JOIN plugin_idc_core_support_ticket_sla_round r ON r.tenant_id=0 AND r.ticket_no=t.ticket_no AND r.round_no=t.current_round WHERE t.tenant_id=0 AND t.base_stage NOT IN ('resolved','closed') AND (r.first_response_due_at IS NOT NULL OR r.resolution_due_at IS NOT NULL OR t.sla_risk_code<>'none') ORDER BY t.id LIMIT ? FOR UPDATE OF t,r SKIP LOCKED`, limit); err != nil {
			return err
		}
		for _, row := range rows {
			firstBreached := row.FirstResponseBreached || timerBreached(now, row.FirstResponseDueAt, row.FirstRespondedAt, false)
			resolutionBreached := row.ResolutionBreached || timerBreached(now, row.ResolutionDueAt, row.ResolvedAt, row.ResolutionTimerIsPaused)
			risk := currentSLARisk(now, row, firstBreached, resolutionBreached)
			if risk == row.Risk && firstBreached == row.FirstResponseBreached && resolutionBreached == row.ResolutionBreached {
				continue
			}
			result, updateErr := w.database.Exec(ctx, `UPDATE plugin_idc_core_support_ticket SET sla_risk_code=?,version=version+1,updated_at=? WHERE tenant_id=0 AND ticket_no=? AND current_round=?`, risk, now, row.TicketNo, row.RoundNo)
			if updateErr != nil {
				return updateErr
			}
			rowsAffected, updateErr := result.RowsAffected()
			if updateErr != nil {
				return updateErr
			}
			if rowsAffected != 1 {
				return gerror.New("support SLA ticket round changed while locked")
			}
			if _, updateErr = w.database.Exec(ctx, `UPDATE plugin_idc_core_support_ticket_sla_round SET first_response_breached=?,resolution_breached=? WHERE tenant_id=0 AND ticket_no=? AND round_no=?`, firstBreached, resolutionBreached, row.TicketNo, row.RoundNo); updateErr != nil {
				return updateErr
			}
			changed++
		}
		return nil
	})
	if err != nil {
		return 0, gerror.Wrap(err, "refresh support SLA risk")
	}
	return changed, nil
}

func timerBreached(now time.Time, dueAt, completedAt *time.Time, paused bool) bool {
	return dueAt != nil && completedAt == nil && !paused && now.After(*dueAt)
}

func currentSLARisk(now time.Time, row slaRiskRow, firstBreached, resolutionBreached bool) string {
	if firstBreached || resolutionBreached {
		return SLARiskBreached
	}
	for _, nodeMinutes := range row.NearDueMinutes {
		if nodeMinutes < 0 {
			continue
		}
		threshold := time.Duration(nodeMinutes) * time.Minute
		if timerAtRisk(now, row.FirstResponseDueAt, row.FirstRespondedAt, false, threshold) ||
			timerAtRisk(now, row.ResolutionDueAt, row.ResolvedAt, row.ResolutionTimerIsPaused, threshold) {
			return SLARiskAtRisk
		}
	}
	return SLARiskNone
}

func timerAtRisk(now time.Time, dueAt, completedAt *time.Time, paused bool, threshold time.Duration) bool {
	return dueAt != nil && completedAt == nil && !paused && !now.After(*dueAt) && !now.Before(dueAt.Add(-threshold))
}
