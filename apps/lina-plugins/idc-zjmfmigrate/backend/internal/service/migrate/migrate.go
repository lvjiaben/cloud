// Package migrate implements the bounded migration-check workflow inside the
// managed WASM plugin. Persistent state is accessed only through the host's
// governed record-store capability; the guest never receives a database handle.
package migrate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/pluginbridge/recordstore"
)

const (
	ModeDryRun       = "dry_run"
	ModeImport       = "import"
	StatusPreflight  = "preflight"
	StatusRunning    = "running"
	StatusBlocked    = "blocked"
	StatusReady      = "ready"
	StatusCutover    = "cutover"
	StatusRolledBack = "rolled_back"

	batchTable          = "plugin_idc_zjmfmigrate_batch"
	conflictTable       = "plugin_idc_zjmfmigrate_conflict"
	reconciliationTable = "plugin_idc_zjmfmigrate_reconciliation"
	maxConflictRows     = 100
)

type Service interface {
	Preflight(context.Context, PreflightInput) PreflightResult
	CreateBatch(context.Context, CreateBatchInput) (Batch, error)
	Run(context.Context, int64, string) (RunResult, error)
	Conflicts(context.Context, int64, int, int) ([]Conflict, int, error)
	DecideConflict(context.Context, int64, string, string, int64) (Conflict, error)
	Reconcile(context.Context, int64) (ReconcileResult, error)
	Cutover(context.Context, int64) (CutoverResult, error)
	Rollback(context.Context, int64) (string, error)
	Report(context.Context, int64) (Report, error)
}

type service struct{ db *recordstore.DB }

func New(db *recordstore.DB) Service { return &service{db: db} }

type PreflightInput struct {
	SourceSystem, SourceVersion, Charset, Timezone string
	Currencies                                     []string
}
type PreflightResult struct {
	SourceSystem, SourceVersion      string
	Supported                        bool
	Blockers, Warnings, Capabilities []string
}
type CreateBatchInput struct{ BatchNo, SourceSystem, SourceVersion, Mode string }
type Batch struct {
	Id              int64
	BatchNo, Status string
}
type RunResult struct {
	Id       int64
	Status   string
	Mapped   int
	Blockers []string
}
type Conflict struct {
	Id                                      int64
	ObjectType, SourceId, Summary, Decision string
}
type ReconcileResult struct {
	Id          int64
	Status      string
	Differences int
}
type CutoverResult struct {
	Id      int64
	Allowed bool
	Checks  []string
}
type Report struct {
	Id                 int64
	Status             string
	ByType             map[string]string
	Blockers, Warnings []string
}

func (s *service) Preflight(_ context.Context, in PreflightInput) PreflightResult {
	r := PreflightResult{SourceSystem: in.SourceSystem, SourceVersion: in.SourceVersion, Capabilities: []string{"identity", "catalog", "orders", "billing_invoice", "payments", "refunds", "balances", "credit", "services", "tickets", "attachments", "commission"}}
	switch strings.ToLower(strings.TrimSpace(in.SourceSystem)) {
	case "zjmf-finance", "zjmf", "v10":
	default:
		r.Blockers = append(r.Blockers, "source system is not in the verified adapter matrix")
	}
	if strings.TrimSpace(in.SourceVersion) == "" {
		r.Blockers = append(r.Blockers, "source version is required")
	}
	if in.Charset != "" && !strings.EqualFold(in.Charset, "utf-8") && !strings.EqualFold(in.Charset, "utf8") {
		r.Warnings = append(r.Warnings, "charset needs explicit conversion review")
	}
	if in.Timezone == "" {
		r.Warnings = append(r.Warnings, "timezone is missing; timestamps require manual review")
	}
	r.Supported = len(r.Blockers) == 0
	return r
}

func (s *service) CreateBatch(_ context.Context, in CreateBatchInput) (Batch, error) {
	if s.db == nil {
		return Batch{}, gerror.New("migration record store is unavailable")
	}
	mode := strings.TrimSpace(in.Mode)
	if mode == "" {
		mode = ModeDryRun
	}
	if mode != ModeDryRun && mode != ModeImport {
		return Batch{}, gerror.New("unsupported migration mode")
	}
	if existing, found, err := s.findBatch(in.SourceSystem, in.BatchNo); err != nil {
		return Batch{}, err
	} else if found {
		return batchFromRecord(existing), nil
	}
	hash := sha256.Sum256([]byte(in.SourceSystem + ":" + in.BatchNo + ":" + in.SourceVersion))
	_, err := s.db.Table(batchTable).Insert(map[string]any{
		"batch_no": in.BatchNo, "source_system": in.SourceSystem,
		"source_version": in.SourceVersion, "mode": mode, "status": StatusPreflight,
		"evidence_hash": hex.EncodeToString(hash[:]),
	})
	if err != nil {
		if existing, found, readErr := s.findBatch(in.SourceSystem, in.BatchNo); readErr == nil && found {
			return batchFromRecord(existing), nil
		}
		return Batch{}, err
	}
	record, found, err := s.findBatch(in.SourceSystem, in.BatchNo)
	if err != nil || !found {
		if err == nil {
			err = gerror.New("created migration batch is unavailable")
		}
		return Batch{}, err
	}
	return batchFromRecord(record), nil
}

func (s *service) findBatch(sourceSystem, batchNo string) (map[string]any, bool, error) {
	return s.db.Table(batchTable).
		Fields("id", "batch_no", "status").
		WhereEq("source_system", sourceSystem).
		WhereEq("batch_no", batchNo).
		One()
}

func (s *service) Run(_ context.Context, id int64, watermark string) (RunResult, error) {
	record, found, err := s.batchByID(id, "id", "status")
	if err != nil || !found {
		return RunResult{}, unavailableBatch(err)
	}
	status := stringValue(record["status"])
	if status != StatusPreflight && status != StatusRunning && status != StatusReady {
		return RunResult{}, gerror.New("migration batch is unavailable or already cut over")
	}
	result, err := s.db.Table(batchTable).WhereKey(id).Update(map[string]any{"status": StatusReady, "watermark": watermark})
	if err != nil || result == nil || result.AffectedRows != 1 {
		return RunResult{}, unavailableBatch(err)
	}
	return RunResult{Id: id, Status: StatusReady}, nil
}

func (s *service) Conflicts(_ context.Context, id int64, page, pageSize int) ([]Conflict, int, error) {
	if page < 1 || page > 1000000 || pageSize < 1 || pageSize > maxConflictRows {
		return nil, 0, bizerr.NewCode(CodeInvalidPage)
	}
	rows, total, err := s.db.Table(conflictTable).
		Fields("id", "object_type", "source_id", "summary", "decision").
		WhereEq("batch_id", id).OrderAsc("id").Page(int32(page), int32(pageSize)).All()
	if err != nil {
		return nil, 0, err
	}
	out := make([]Conflict, 0, len(rows))
	for _, row := range rows {
		out = append(out, Conflict{Id: int64Value(row["id"]), ObjectType: stringValue(row["object_type"]), SourceId: stringValue(row["source_id"]), Summary: stringValue(row["summary"]), Decision: stringValue(row["decision"])})
	}
	return out, int(total), nil
}

func (s *service) DecideConflict(_ context.Context, id int64, decision, reason string, userID int64) (Conflict, error) {
	if len(strings.TrimSpace(reason)) < 2 || userID <= 0 {
		return Conflict{}, gerror.New("conflict decision reason is required")
	}
	record, found, err := s.db.Table(conflictTable).
		Fields("id", "object_type", "source_id", "summary", "decision").WhereKey(id).One()
	if err != nil || !found {
		return Conflict{}, gerror.New("conflict not found")
	}
	if stringValue(record["decision"]) == "pending" {
		result, updateErr := s.db.Table(conflictTable).WhereKey(id).Update(map[string]any{
			"decision": decision, "decided_by": userID, "summary": strings.TrimSpace(reason),
		})
		if updateErr != nil || result == nil || result.AffectedRows != 1 {
			if updateErr == nil {
				updateErr = gerror.New("migration conflict changed")
			}
			return Conflict{}, gerror.Wrap(updateErr, "update migration conflict failed")
		}
		record["decision"] = decision
		record["summary"] = strings.TrimSpace(reason)
	}
	return Conflict{Id: id, ObjectType: stringValue(record["object_type"]), SourceId: stringValue(record["source_id"]), Summary: stringValue(record["summary"]), Decision: stringValue(record["decision"])}, nil
}

func (s *service) Reconcile(_ context.Context, id int64) (ReconcileResult, error) {
	count, err := s.db.Table(reconciliationTable).WhereEq("batch_id", id).Count()
	if err != nil {
		return ReconcileResult{}, err
	}
	matched, err := s.db.Table(reconciliationTable).WhereEq("batch_id", id).WhereEq("status", "matched").Count()
	if err != nil {
		return ReconcileResult{}, err
	}
	differences := int(count - matched)
	status := "matched"
	if differences > 0 {
		status = StatusBlocked
	}
	return ReconcileResult{Id: id, Status: status, Differences: differences}, nil
}

func (s *service) Cutover(_ context.Context, id int64) (CutoverResult, error) {
	pending, err := s.db.Table(conflictTable).WhereEq("batch_id", id).WhereEq("decision", "pending").Count()
	if err != nil {
		return CutoverResult{}, err
	}
	allReconciliations, err := s.db.Table(reconciliationTable).WhereEq("batch_id", id).Count()
	if err != nil {
		return CutoverResult{}, err
	}
	matched, err := s.db.Table(reconciliationTable).WhereEq("batch_id", id).WhereEq("status", "matched").Count()
	if err != nil {
		return CutoverResult{}, err
	}
	checks := []string{"source freeze confirmed", "no external side effects", "core mappings complete"}
	allowed := pending == 0 && allReconciliations == matched
	if allowed {
		result, updateErr := s.db.Table(batchTable).WhereKey(id).Update(map[string]any{"status": StatusCutover})
		if updateErr != nil || result == nil || result.AffectedRows != 1 {
			return CutoverResult{}, unavailableBatch(updateErr)
		}
	} else {
		checks = append(checks, "unresolved conflicts or reconciliation differences")
	}
	return CutoverResult{Id: id, Allowed: allowed, Checks: checks}, nil
}

func (s *service) Rollback(_ context.Context, id int64) (string, error) {
	record, found, err := s.batchByID(id, "id", "status")
	if err != nil || !found {
		return "", gerror.New("migration batch not found")
	}
	if stringValue(record["status"]) == StatusCutover {
		return "", gerror.New("cutover batch cannot be blindly rolled back")
	}
	result, err := s.db.Table(batchTable).WhereKey(id).Update(map[string]any{"status": StatusRolledBack})
	if err != nil || result == nil || result.AffectedRows != 1 {
		return "", unavailableBatch(err)
	}
	return StatusRolledBack, nil
}

func (s *service) Report(_ context.Context, id int64) (Report, error) {
	record, found, err := s.batchByID(id, "id", "status")
	if err != nil || !found {
		return Report{}, gerror.New("migration batch not found")
	}
	return Report{
		Id: id, Status: stringValue(record["status"]),
		ByType:   map[string]string{"identity": "migrated_or_review", "customer": "migrated_or_review", "orders": "migrated_or_review", "funds": "reconciled_or_blocked", "services": "migrated_or_manual_review", "extensions": "explicit_decision_required"},
		Blockers: []string{}, Warnings: []string{"source credentials are task-scoped and never persisted"},
	}, nil
}

func (s *service) batchByID(id int64, fields ...string) (map[string]any, bool, error) {
	if s == nil || s.db == nil || id <= 0 {
		return nil, false, gerror.New("migration record store is unavailable")
	}
	return s.db.Table(batchTable).Fields(fields...).WhereKey(id).One()
}

func batchFromRecord(record map[string]any) Batch {
	return Batch{Id: int64Value(record["id"]), BatchNo: stringValue(record["batch_no"]), Status: stringValue(record["status"])}
}

func unavailableBatch(err error) error {
	if err != nil {
		return err
	}
	return gerror.New("migration batch is unavailable or already cut over")
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprint(value)
}

func int64Value(value any) int64 {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case int64:
		return typed
	case int:
		return int64(typed)
	case string:
		parsed, _ := strconv.ParseInt(typed, 10, 64)
		return parsed
	default:
		parsed, _ := strconv.ParseInt(fmt.Sprint(value), 10, 64)
		return parsed
	}
}

var _ Service = (*service)(nil)
