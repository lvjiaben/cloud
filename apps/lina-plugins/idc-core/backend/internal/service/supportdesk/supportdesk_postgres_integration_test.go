package supportdesk

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

const supportDeskIntegrationDSNEnv = "LINAPRO_IDC_CORE_TEST_DSN"

func TestSLAWorkerPersistsRiskAndBreachesExactlyOncePostgreSQL(t *testing.T) {
	ctx, database := newSupportDeskTestDatabase(t)
	seedSupportSLA(t, ctx, database)
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket (tenant_id,ticket_no,ticket_type,visibility_code,subject,base_stage,priority_code,current_round,last_activity_at,created_by,updated_by) VALUES (0,'TKT_sla','internal','internal','SLA','in_progress','normal',1,CURRENT_TIMESTAMP,7,7)`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket_sla_round (tenant_id,ticket_no,round_no,rule_no,rule_version,calendar_no,calendar_version,calendar_mode,near_due_minutes,first_response_minutes,resolution_minutes,first_response_due_at,resolution_due_at,created_at) VALUES (0,'TKT_sla',1,'SLR_default',1,'SLC_default',1,'continuous',ARRAY[60,15],120,240,CURRENT_TIMESTAMP+INTERVAL '30 minutes',CURRENT_TIMESTAMP+INTERVAL '3 hours',CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	worker, err := NewSLAWorker(database)
	if err != nil {
		t.Fatal(err)
	}
	if changed, runErr := worker.RefreshRisk(ctx, 10); runErr != nil || changed != 1 {
		t.Fatalf("first refresh changed=%d err=%v", changed, runErr)
	}
	assertTicketRisk(t, ctx, database, "TKT_sla", SLARiskAtRisk, 2)
	if changed, runErr := worker.RefreshRisk(ctx, 10); runErr != nil || changed != 0 {
		t.Fatalf("idempotent refresh changed=%d err=%v", changed, runErr)
	}
	assertTicketRisk(t, ctx, database, "TKT_sla", SLARiskAtRisk, 2)
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_support_ticket_sla_round SET first_response_due_at=CURRENT_TIMESTAMP-INTERVAL '1 second' WHERE ticket_no='TKT_sla' AND round_no=1`); err != nil {
		t.Fatal(err)
	}
	if changed, runErr := worker.RefreshRisk(ctx, 10); runErr != nil || changed != 1 {
		t.Fatalf("breach refresh changed=%d err=%v", changed, runErr)
	}
	assertTicketRisk(t, ctx, database, "TKT_sla", SLARiskBreached, 3)
	breachedValue, err := database.GetValue(ctx, `SELECT first_response_breached FROM plugin_idc_core_support_ticket_sla_round WHERE ticket_no='TKT_sla' AND round_no=1`)
	if err != nil || !breachedValue.Bool() {
		t.Fatalf("first response breach=%v err=%v", breachedValue.Bool(), err)
	}
}

func TestSLAWorkerDoesNotAdvancePausedResolutionTimerPostgreSQL(t *testing.T) {
	ctx, database := newSupportDeskTestDatabase(t)
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket (tenant_id,ticket_no,ticket_type,visibility_code,subject,base_stage,priority_code,current_round,last_activity_at,created_by,updated_by) VALUES (0,'TKT_paused','internal','internal','Paused','waiting_customer','normal',1,CURRENT_TIMESTAMP,7,7)`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket_sla_round (tenant_id,ticket_no,round_no,near_due_minutes,resolution_minutes,resolution_due_at,created_at) VALUES (0,'TKT_paused',1,ARRAY[60],120,CURRENT_TIMESTAMP-INTERVAL '1 minute',CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket_sla_pause (tenant_id,ticket_no,round_no,started_at,reason_code) VALUES (0,'TKT_paused',1,CURRENT_TIMESTAMP-INTERVAL '5 minutes','waiting_customer')`); err != nil {
		t.Fatal(err)
	}
	worker, err := NewSLAWorker(database)
	if err != nil {
		t.Fatal(err)
	}
	if changed, runErr := worker.RefreshRisk(ctx, 10); runErr != nil || changed != 0 {
		t.Fatalf("paused refresh changed=%d err=%v", changed, runErr)
	}
	assertTicketRisk(t, ctx, database, "TKT_paused", SLARiskNone, 1)
}

func TestAutomationWorkerClosesOnlyEligibleResolvedTicketsPostgreSQL(t *testing.T) {
	ctx, database := newSupportDeskTestDatabase(t)
	for _, statement := range []string{
		`INSERT INTO plugin_idc_core_support_ticket (tenant_id,ticket_no,ticket_type,visibility_code,subject,base_stage,priority_code,current_round,resolved_at,last_activity_at,created_by,updated_by) VALUES (0,'TKT_auto_close','internal','internal','Eligible','resolved','normal',1,CURRENT_TIMESTAMP-INTERVAL '4 days',CURRENT_TIMESTAMP,7,7)`,
		`INSERT INTO plugin_idc_core_support_ticket (tenant_id,ticket_no,ticket_type,visibility_code,subject,base_stage,priority_code,current_round,resolved_at,last_activity_at,created_by,updated_by) VALUES (0,'TKT_recent_resolved','internal','internal','Recent','resolved','normal',1,CURRENT_TIMESTAMP-INTERVAL '2 days',CURRENT_TIMESTAMP,7,7)`,
		`INSERT INTO plugin_idc_core_support_ticket (tenant_id,ticket_no,ticket_type,visibility_code,subject,base_stage,priority_code,current_round,resolved_at,customer_last_replied_at,last_activity_at,created_by,updated_by) VALUES (0,'TKT_replied_after_resolution','customer','customer','Replied','resolved','normal',1,CURRENT_TIMESTAMP-INTERVAL '4 days',CURRENT_TIMESTAMP-INTERVAL '3 days',CURRENT_TIMESTAMP,7,7)`,
	} {
		if _, err := database.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	worker, err := NewAutomationWorker(database)
	if err != nil {
		t.Fatal(err)
	}
	if closed, runErr := worker.AutoCloseResolved(ctx, 10); runErr != nil || closed != 1 {
		t.Fatalf("first auto-close count=%d err=%v", closed, runErr)
	}
	if closed, runErr := worker.AutoCloseResolved(ctx, 10); runErr != nil || closed != 0 {
		t.Fatalf("idempotent auto-close count=%d err=%v", closed, runErr)
	}
	var rows []struct {
		TicketNo string `orm:"ticket_no"`
		Stage    string `orm:"base_stage"`
		Reason   string `orm:"closed_reason_code"`
		Version  int64  `orm:"version"`
	}
	if err = database.GetScan(ctx, &rows, `SELECT ticket_no,base_stage,COALESCE(closed_reason_code,'') AS closed_reason_code,version FROM plugin_idc_core_support_ticket WHERE ticket_no LIKE 'TKT_%' ORDER BY ticket_no`); err != nil {
		t.Fatal(err)
	}
	states := make(map[string]struct {
		Stage, Reason string
		Version       int64
	}, len(rows))
	for _, row := range rows {
		states[row.TicketNo] = struct {
			Stage, Reason string
			Version       int64
		}{row.Stage, row.Reason, row.Version}
	}
	if got := states["TKT_auto_close"]; got.Stage != "closed" || got.Reason != "auto_close" || got.Version != 2 {
		t.Fatalf("eligible auto-close state=%+v", got)
	}
	if got := states["TKT_recent_resolved"]; got.Stage != "resolved" || got.Version != 1 {
		t.Fatalf("recent resolved ticket changed=%+v", got)
	}
	if got := states["TKT_replied_after_resolution"]; got.Stage != "resolved" || got.Version != 1 {
		t.Fatalf("ticket with newer customer reply changed=%+v", got)
	}
	history, err := queryCount(ctx, database, `SELECT COUNT(1) FROM plugin_idc_core_support_ticket_status_history WHERE ticket_no='TKT_auto_close' AND source_code='automation' AND reason_code='auto_close' AND rule_version=1`)
	if err != nil || history != 1 {
		t.Fatalf("auto-close history count=%d err=%v", history, err)
	}
}

func TestStaffClosePersistsTransitionWithCompleteSQLArgumentsPostgreSQL(t *testing.T) {
	ctx, database := newSupportDeskTestDatabase(t)
	service := newAuthorizedSupportDeskTestService(database)
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket
        (tenant_id,ticket_no,ticket_type,visibility_code,subject,base_stage,priority_code,current_round,last_activity_at,created_by,updated_by)
        VALUES (0,'TKT_staff_close','internal','internal','Close regression','in_progress','normal',1,CURRENT_TIMESTAMP,7,7)`); err != nil {
		t.Fatal(err)
	}

	detail, err := service.CloseStaff(ctx, TransitionInput{
		TicketNo: "TKT_staff_close", TicketVersion: 1, Reason: "verified resolution",
	})
	if err != nil {
		t.Fatalf("close staff ticket: %v", err)
	}
	if detail.Ticket.BaseStage != "closed" || detail.Ticket.Version != 2 {
		t.Fatalf("unexpected closed ticket: %+v", detail.Ticket)
	}
	var closure struct {
		ClosedByType     string `json:"closed_by_type"`
		ClosedReasonCode string `json:"closed_reason_code"`
	}
	if err = database.GetScan(ctx, &closure, `SELECT closed_reason_code,closed_by_type
        FROM plugin_idc_core_support_ticket WHERE tenant_id=0 AND ticket_no='TKT_staff_close'`); err != nil {
		t.Fatal(err)
	}
	if closure.ClosedReasonCode != "forced" || closure.ClosedByType != "leader" {
		t.Fatalf("unexpected persisted closure metadata: %+v", closure)
	}
	var historyCount int
	value, err := database.GetValue(ctx, `SELECT COUNT(1) FROM plugin_idc_core_support_ticket_status_history
        WHERE ticket_no='TKT_staff_close' AND from_base_stage='in_progress' AND to_base_stage='closed'
          AND source_code='leader' AND reason_code='forced'`)
	if err != nil {
		t.Fatal(err)
	}
	historyCount = value.Int()
	if historyCount != 1 {
		t.Fatalf("close status history count=%d, want 1", historyCount)
	}
}

func TestMergeUsesMainAndSourceVersionFencesPostgreSQL(t *testing.T) {
	ctx, database := newSupportDeskTestDatabase(t)
	service := newAuthorizedSupportDeskTestService(database)
	for _, ticketNo := range []string{"TKT_merge_main", "TKT_merge_source"} {
		if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket (tenant_id,ticket_no,ticket_type,visibility_code,subject,base_stage,priority_code,current_round,last_activity_at,created_by,updated_by) VALUES (0,?,'internal','internal',?,'in_progress','normal',1,CURRENT_TIMESTAMP,7,7)`, ticketNo, ticketNo); err != nil {
			t.Fatal(err)
		}
	}
	input := MergeInput{
		Main:    VersionedTicket{TicketNo: "TKT_merge_main", Version: 1},
		Sources: []VersionedTicket{{TicketNo: "TKT_merge_source", Version: 1}},
		Reason:  "same incident",
	}
	var wait sync.WaitGroup
	errors := make(chan error, 2)
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, err := service.Merge(ctx, input, true)
			errors <- err
		}()
	}
	wait.Wait()
	close(errors)
	var succeeded, conflicted int
	for err := range errors {
		switch {
		case err == nil:
			succeeded++
		case bizerr.Is(err, CodeConflict):
			conflicted++
		default:
			t.Fatalf("unexpected merge result: %v", err)
		}
	}
	if succeeded != 1 || conflicted != 1 {
		t.Fatalf("merge results success/conflict=%d/%d, want 1/1", succeeded, conflicted)
	}
	var rows []struct {
		TicketNo string `orm:"ticket_no"`
		Stage    string `orm:"base_stage"`
		Version  int64  `orm:"version"`
	}
	if err := database.GetScan(ctx, &rows, `SELECT ticket_no,base_stage,version FROM plugin_idc_core_support_ticket WHERE ticket_no IN ('TKT_merge_main','TKT_merge_source') ORDER BY ticket_no`); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].TicketNo != "TKT_merge_main" || rows[0].Stage != "in_progress" || rows[0].Version != 2 || rows[1].TicketNo != "TKT_merge_source" || rows[1].Stage != "closed" || rows[1].Version != 2 {
		t.Fatalf("unexpected merged ticket fences: %+v", rows)
	}
	relations, err := queryCount(ctx, database, `SELECT COUNT(1) FROM plugin_idc_core_support_ticket_relation WHERE source_ticket_no='TKT_merge_source' AND target_ticket_no='TKT_merge_main'`)
	if err != nil || relations != 1 {
		t.Fatalf("merge relation count=%d err=%v", relations, err)
	}
}

func TestInitialAttachmentReservationBindsOnlyWhenConsumedPostgreSQL(t *testing.T) {
	ctx, database := newSupportDeskTestDatabase(t)
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket (tenant_id,ticket_no,ticket_type,visibility_code,subject,base_stage,priority_code,current_round,last_activity_at,created_by,updated_by) VALUES (0,'TKT_initial_upload','internal','internal','Initial attachment','new','normal',1,CURRENT_TIMESTAMP,7,7)`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket_attachment_upload (tenant_id,upload_no,ticket_no,uploader_user_id,uploader_type,context_code,file_name,declared_size_bytes,temporary_object_ref,media_type,sha256_hex,status_code,expires_at,completed_at) VALUES (0,'TUP_initial',NULL,7,'staff','internal_initial','note.txt',4,'tmp/note','text/plain',repeat('a',64),'ready',CURRENT_TIMESTAMP+INTERVAL '1 hour',CURRENT_TIMESTAMP)`); err != nil {
		t.Fatalf("insert unbound initial reservation: %v", err)
	}
	result, err := database.Exec(ctx, `UPDATE plugin_idc_core_support_ticket_attachment_upload SET ticket_no='TKT_initial_upload',status_code='consumed',consumed_at=CURRENT_TIMESTAMP WHERE upload_no='TUP_initial' AND ticket_no IS NULL`)
	if err != nil {
		t.Fatalf("bind consumed initial reservation: %v", err)
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		t.Fatalf("consumed initial reservation changed rows=%d, want 1", changed)
	}
}

type supportDeskBizCtxStub struct{}

func (supportDeskBizCtxStub) Current(context.Context) bizctxcap.CurrentContext {
	return bizctxcap.CurrentContext{UserID: 7, TenantID: 0, DataScope: 1}
}

type supportDeskPrincipalStub struct{}

func (supportDeskPrincipalStub) Current(context.Context) (*principal.Audience, error) {
	return &principal.Audience{UserID: 7, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive}, nil
}

type supportDeskAuthzStub struct{}

func (supportDeskAuthzStub) BatchGetPermissions(context.Context, []authz.PermissionKey) (*capmodel.BatchResult[*authz.PermissionInfo, authz.PermissionKey], error) {
	return nil, nil
}
func (supportDeskAuthzStub) BatchHasPermissions(_ context.Context, keys []authz.PermissionKey) (map[authz.PermissionKey]bool, error) {
	result := make(map[authz.PermissionKey]bool, len(keys))
	for _, key := range keys {
		result[key] = true
	}
	return result, nil
}
func (supportDeskAuthzStub) HasPermission(context.Context, authz.PermissionKey) (bool, error) {
	return true, nil
}
func (supportDeskAuthzStub) IsPlatformAdmin(context.Context, authz.UserID) (bool, error) {
	return true, nil
}
func (supportDeskAuthzStub) ReplaceRolePermissions(context.Context, authz.RoleID, []authz.PermissionKey) error {
	return nil
}

func newAuthorizedSupportDeskTestService(database gdb.DB) *serviceImpl {
	return &serviceImpl{database: database, bizCtx: supportDeskBizCtxStub{}, principal: supportDeskPrincipalStub{}, authorization: supportDeskAuthzStub{}, random: rand.Reader}
}

func seedSupportSLA(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_support_department (tenant_id,department_no,name_zh_cn,name_en_us,assignment_mode,status_code,version,created_by,updated_by) VALUES (0,'DEP_support','Support','Support','manual_claim','active',1,7,7)`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_support_sla_calendar (tenant_id,calendar_no,name,mode_code,status_code,version,created_by,updated_by) VALUES (0,'SLC_default','Always','continuous','active',1,7,7)`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_support_sla_rule (tenant_id,rule_no,department_no,priority_code,calendar_no,first_response_minutes,resolution_minutes,near_due_minutes,status_code,version,created_by,updated_by) VALUES (0,'SLR_default','DEP_support','normal','SLC_default',120,240,ARRAY[60,15],'active',1,7,7)`); err != nil {
		t.Fatal(err)
	}
}

func assertTicketRisk(t *testing.T, ctx context.Context, database gdb.DB, ticketNo, risk string, version int64) {
	t.Helper()
	var row struct {
		Risk    string `orm:"sla_risk_code"`
		Version int64  `orm:"version"`
	}
	if err := database.GetScan(ctx, &row, `SELECT sla_risk_code,version FROM plugin_idc_core_support_ticket WHERE ticket_no=?`, ticketNo); err != nil {
		t.Fatal(err)
	}
	if row.Risk != risk || row.Version != version {
		t.Fatalf("ticket %s risk/version = %s/%d, want %s/%d", ticketNo, row.Risk, row.Version, risk, version)
	}
}

func newSupportDeskTestDatabase(t *testing.T) (context.Context, gdb.DB) {
	t.Helper()
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv(supportDeskIntegrationDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run support desk PostgreSQL tests", supportDeskIntegrationDSNEnv)
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	var suffix [8]byte
	if _, err = rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	schema := "linapro_supportdesk_" + hex.EncodeToString(suffix[:])
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn + separator + "search_path=" + url.QueryEscape(schema)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = database.Close(ctx)
		if strings.HasPrefix(schema, "linapro_supportdesk_") {
			_, _ = admin.Exec(ctx, `DROP SCHEMA "`+schema+`" CASCADE`)
		}
		_ = admin.Close(ctx)
	})
	if _, err = database.Exec(ctx, `CREATE TABLE plugin_idc_core_customer_subject (tenant_id BIGINT NOT NULL DEFAULT 0, customer_no VARCHAR(40) PRIMARY KEY, display_name VARCHAR(160) NOT NULL DEFAULT '')`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `CREATE TABLE plugin_idc_core_catalog_product (id BIGSERIAL PRIMARY KEY, tenant_id BIGINT NOT NULL DEFAULT 0, product_no VARCHAR(40) NOT NULL UNIQUE, name_zh_cn VARCHAR(256) NOT NULL, name_en_us VARCHAR(256) NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `CREATE TABLE plugin_idc_core_product_fulfillment_intent (tenant_id BIGINT NOT NULL DEFAULT 0, intent_no VARCHAR(40) NOT NULL UNIQUE, product_name_zh_cn VARCHAR(256) NOT NULL, product_name_en_us VARCHAR(256) NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `CREATE TABLE plugin_idc_core_service_entitlement (id BIGSERIAL PRIMARY KEY, tenant_id BIGINT NOT NULL DEFAULT 0, entitlement_no VARCHAR(40) NOT NULL UNIQUE, source_intent_no VARCHAR(40) NOT NULL, customer_no VARCHAR(40) NOT NULL, status_code VARCHAR(24) NOT NULL, version BIGINT NOT NULL DEFAULT 1)`); err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "manifest", "sql"))
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(root, "042-support-content.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, string(content)); err != nil {
		t.Fatalf("execute support/content migration: %v", err)
	}
	upstreamContent, err := os.ReadFile(filepath.Join(root, "060-support-upstream-channel.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, string(upstreamContent)); err != nil {
		t.Fatalf("execute support upstream migration: %v", err)
	}
	entitlementContent, err := os.ReadFile(filepath.Join(root, "091-support-entitlement-association.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for attempt := 1; attempt <= 2; attempt++ {
		if _, err = database.Exec(ctx, string(entitlementContent)); err != nil {
			t.Fatalf("execute support entitlement migration attempt %d: %v", attempt, err)
		}
	}
	queueMigration, err := os.ReadFile(filepath.Join(root, "113-support-role-organization.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, string(queueMigration)); err != nil {
		t.Fatal(err)
	}
	return ctx, database
}
