// This file verifies anonymous presales idempotency, fixed access links, and public custom-field snapshots against PostgreSQL.
package supportdesk

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/supportcap"
)

func TestVerifiedAnonymousCreateIsIdempotentAndPersistsNoPlaintextPostgreSQL(t *testing.T) {
	ctx, database := newSupportDeskTestDatabase(t)
	seedVerifiedAnonymousRoute(t, ctx, database)
	manager, accessor := newVerifiedAnonymousTokenStubs(t)
	service, err := NewVerifiedAnonymousCreator(database, manager, accessor)
	if err != nil {
		t.Fatal(err)
	}
	request := verifiedAnonymousRequest("operation:verified:1")
	first, err := service.CreateAnonymousPresales(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.CreateAnonymousPresales(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if first.TicketNo != second.TicketNo || first.AccessToken != second.AccessToken ||
		!first.ExpiresAt.Equal(second.ExpiresAt) || first.AccessToken != verifiedAnonymousTestToken {
		t.Fatalf("idempotent result changed: first=%+v second=%+v", first, second)
	}
	assertVerifiedAnonymousRows(t, ctx, database, request, first)

	changed := request
	changed.Subject = "A different question"
	if _, err = service.CreateAnonymousPresales(ctx, changed); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("same operation with changed content error=%v, want conflict", err)
	}
}

func TestVerifiedAnonymousCreateValidatesAndSnapshotsPublicCustomFieldsPostgreSQL(t *testing.T) {
	ctx, database := newSupportDeskTestDatabase(t)
	seedVerifiedAnonymousRoute(t, ctx, database)
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_support_custom_field_definition (tenant_id,field_no,field_type,name_zh_cn,name_en_us,description_zh_cn,description_en_us,applies_presales,visibility_code,required,sort_order,status_code,version) VALUES (0,'SCF_presales_need','multi_select','咨询方向','Question area','选择咨询方向','Select the question area',TRUE,'customer_visible',TRUE,10,'active',3)`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_support_custom_field_option (tenant_id,option_no,field_no,name_zh_cn,name_en_us,sort_order,status_code,version) VALUES (0,'SCO_compute','SCF_presales_need','计算','Compute',1,'active',2),(0,'SCO_network','SCF_presales_need','网络','Network',2,'active',4)`); err != nil {
		t.Fatal(err)
	}
	manager, accessor := newVerifiedAnonymousTokenStubs(t)
	service, err := NewVerifiedAnonymousCreator(database, manager, accessor)
	if err != nil {
		t.Fatal(err)
	}
	request := verifiedAnonymousRequest("operation:verified:custom-fields")
	request.CustomFields = []supportcap.CustomFieldValue{{
		FieldNo: "SCF_presales_need", OptionNos: []string{"SCO_network", "SCO_compute"},
	}}
	result, err := service.CreateAnonymousPresales(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	var value struct {
		FieldType, NameZhCN, NameEnUS, DescriptionZhCN, DescriptionEnUS string
		Visibility, NormalizedValue                                     string
		DefinitionVersion, SelectedOptionCount                          int64
	}
	if err = database.GetScan(ctx, &value, `SELECT field_type,name_zh_cn,name_en_us,description_zh_cn,description_en_us,visibility_code AS visibility,normalized_value,definition_version,selected_option_count FROM plugin_idc_core_support_ticket_custom_field_value WHERE tenant_id=0 AND ticket_no=? AND field_no='SCF_presales_need'`, result.TicketNo); err != nil {
		t.Fatal(err)
	}
	if value.FieldType != customFieldMultiSelect || value.NameZhCN != "咨询方向" ||
		value.NameEnUS != "Question area" || value.Visibility != customFieldCustomer ||
		value.NormalizedValue != "SCO_compute,SCO_network" || value.DefinitionVersion != 3 ||
		value.SelectedOptionCount != 2 {
		t.Fatalf("unexpected anonymous custom-field snapshot: %+v", value)
	}
	optionCount, err := queryCount(ctx, database, `SELECT COUNT(1) FROM plugin_idc_core_support_ticket_custom_field_value_option WHERE tenant_id=0 AND ticket_no=? AND field_no='SCF_presales_need' AND option_no IN ('SCO_compute','SCO_network')`, result.TicketNo)
	if err != nil || optionCount != 2 {
		t.Fatalf("anonymous option snapshots=%d err=%v", optionCount, err)
	}

	missing := verifiedAnonymousRequest("operation:verified:missing-custom-field")
	if _, err = service.CreateAnonymousPresales(ctx, missing); !bizerr.Is(err, CodeInvalid) {
		t.Fatalf("missing required anonymous custom field error=%v, want invalid", err)
	}
	ticketCount, err := queryCount(ctx, database, `SELECT COUNT(1) FROM plugin_idc_core_support_ticket`)
	if err != nil || ticketCount != 1 {
		t.Fatalf("invalid anonymous create wrote a ticket: count=%d err=%v", ticketCount, err)
	}
}

func TestAnonymousCreateRequestDigestCanonicalizesCustomFieldOrder(t *testing.T) {
	text := "same"
	first := verifiedAnonymousRequest("operation:digest:custom-fields")
	first.CustomFields = []supportcap.CustomFieldValue{
		{FieldNo: "SCF_text", TextValue: &text},
		{FieldNo: "SCF_options", OptionNos: []string{"SCO_two", "SCO_one"}},
	}
	second := first
	second.CustomFields = []supportcap.CustomFieldValue{
		{FieldNo: "SCF_options", OptionNos: []string{"SCO_one", "SCO_two"}},
		{FieldNo: "SCF_text", TextValue: &text},
	}
	firstDigest, err := anonymousCreateRequestDigest(first)
	if err != nil {
		t.Fatal(err)
	}
	secondDigest, err := anonymousCreateRequestDigest(second)
	if err != nil {
		t.Fatal(err)
	}
	if firstDigest != secondDigest {
		t.Fatalf("equivalent custom-field submissions changed digest: %s != %s", firstDigest, secondDigest)
	}
}

func TestVerifiedAnonymousCreateRecoversAfterTokenPreparationFailurePostgreSQL(t *testing.T) {
	ctx, database := newSupportDeskTestDatabase(t)
	seedVerifiedAnonymousRoute(t, ctx, database)
	manager, accessor := newVerifiedAnonymousTokenStubs(t)
	accessor.failuresBefore = 1
	service, err := NewVerifiedAnonymousCreator(database, manager, accessor)
	if err != nil {
		t.Fatal(err)
	}
	request := verifiedAnonymousRequest("operation:verified:recovery")
	if _, err = service.CreateAnonymousPresales(ctx, request); !bizerr.Is(err, CodeUnavailable) {
		t.Fatalf("interrupted token access error=%v, want unavailable", err)
	}
	count, err := queryCount(ctx, database, `SELECT COUNT(1) FROM plugin_idc_core_support_ticket WHERE tenant_id=0`)
	if err != nil || count != 0 {
		t.Fatalf("failed preparation created ticket: count=%d err=%v", count, err)
	}
	recovered, err := service.CreateAnonymousPresales(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.AccessToken != verifiedAnonymousTestToken {
		t.Fatalf("recovered token=%q", recovered.AccessToken)
	}
	assertVerifiedAnonymousRows(t, ctx, database, request, recovered)
}

func TestVerifiedAnonymousCreateConcurrentReplayHasOneTicketPostgreSQL(t *testing.T) {
	ctx, database := newSupportDeskTestDatabase(t)
	seedVerifiedAnonymousRoute(t, ctx, database)
	manager, accessor := newVerifiedAnonymousTokenStubs(t)
	service, err := NewVerifiedAnonymousCreator(database, manager, accessor)
	if err != nil {
		t.Fatal(err)
	}
	request := verifiedAnonymousRequest("operation:verified:concurrent")
	const callers = 8
	results := make([]*supportcap.CreateAnonymousPresalesResult, callers)
	errorsByCaller := make([]error, callers)
	var wait sync.WaitGroup
	for index := range callers {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			results[index], errorsByCaller[index] = service.CreateAnonymousPresales(context.Background(), request)
		}(index)
	}
	wait.Wait()
	for index := range callers {
		if errorsByCaller[index] != nil || results[index] == nil {
			t.Fatalf("concurrent caller %d result=%+v err=%v", index, results[index], errorsByCaller[index])
		}
		if results[index].TicketNo != results[0].TicketNo ||
			results[index].AccessToken != results[0].AccessToken ||
			!results[index].ExpiresAt.Equal(results[0].ExpiresAt) {
			t.Fatalf("concurrent caller %d returned a different result", index)
		}
	}
	assertVerifiedAnonymousRows(t, ctx, database, request, results[0])
}

func TestVerifiedAnonymousRenewIsIdempotentAndLeavesExpiredLinkDeadPostgreSQL(t *testing.T) {
	ctx, database := newSupportDeskTestDatabase(t)
	request := verifiedAnonymousRenewRequest("operation:renew:1")
	seedExpiredAnonymousAccess(t, ctx, database, request, false)
	manager, accessor := newVerifiedAnonymousTokenStubs(t)
	service, err := NewVerifiedAnonymousCreator(database, manager, accessor)
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.RenewAnonymousPresalesAccess(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.RenewAnonymousPresalesAccess(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if first.TicketNo != second.TicketNo || first.AccessToken != second.AccessToken ||
		!first.ExpiresAt.Equal(second.ExpiresAt) || first.AccessToken != verifiedAnonymousTestToken {
		t.Fatalf("idempotent renewal changed: first=%+v second=%+v", first, second)
	}
	assertVerifiedAnonymousRenewalRows(t, ctx, database, request, first)
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_support_ticket_access_link SET revoked_at=CURRENT_TIMESTAMP WHERE operation_digest=?`, mustRenewOperationDigest(t, request.OperationNo)); err != nil {
		t.Fatal(err)
	}
	if _, err = service.RenewAnonymousPresalesAccess(ctx, request); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("replay of superseded renewal error=%v, want conflict", err)
	}

	changed := request
	changed.DestinationDigest = strings.Repeat("c", 64)
	if _, err = service.RenewAnonymousPresalesAccess(ctx, changed); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("same renewal operation with changed destination error=%v, want conflict", err)
	}
}

func mustRenewOperationDigest(t *testing.T, operationNo string) string {
	t.Helper()
	digest, err := supportcap.RenewOperationDigest(operationNo)
	if err != nil {
		t.Fatal(err)
	}
	return digest
}

func TestVerifiedAnonymousRenewRevokesAnActiveOldLinkPostgreSQL(t *testing.T) {
	ctx, database := newSupportDeskTestDatabase(t)
	request := verifiedAnonymousRenewRequest("operation:renew:active")
	seedExpiredAnonymousAccess(t, ctx, database, request, true)
	manager, accessor := newVerifiedAnonymousTokenStubs(t)
	service, err := NewVerifiedAnonymousCreator(database, manager, accessor)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.RenewAnonymousPresalesAccess(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	oldRevoked, err := queryCount(ctx, database, `SELECT COUNT(1) FROM plugin_idc_core_support_ticket_access_link WHERE link_no='SAL_expired_original' AND revoked_at IS NOT NULL`)
	if err != nil || oldRevoked != 1 {
		t.Fatalf("active old link was not revoked: count=%d err=%v", oldRevoked, err)
	}
	if result.AccessToken != verifiedAnonymousTestToken {
		t.Fatalf("unexpected renewed token %q", result.AccessToken)
	}
}

func TestVerifiedAnonymousRenewRecoversPreparedTokenAfterInterruptionPostgreSQL(t *testing.T) {
	ctx, database := newSupportDeskTestDatabase(t)
	request := verifiedAnonymousRenewRequest("operation:renew:recovery")
	seedExpiredAnonymousAccess(t, ctx, database, request, false)
	manager, accessor := newVerifiedAnonymousTokenStubs(t)
	accessor.failuresBefore = 1
	service, err := NewVerifiedAnonymousCreator(database, manager, accessor)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.RenewAnonymousPresalesAccess(ctx, request); !bizerr.Is(err, CodeUnavailable) {
		t.Fatalf("interrupted renewal error=%v, want unavailable", err)
	}
	count, err := queryCount(ctx, database, `SELECT COUNT(1) FROM plugin_idc_core_support_ticket_access_link`)
	if err != nil || count != 1 {
		t.Fatalf("interrupted renewal changed links: count=%d err=%v", count, err)
	}
	recovered, err := service.RenewAnonymousPresalesAccess(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	assertVerifiedAnonymousRenewalRows(t, ctx, database, request, recovered)
}

func TestVerifiedAnonymousRenewConcurrentReplayHasOneNewLinkPostgreSQL(t *testing.T) {
	ctx, database := newSupportDeskTestDatabase(t)
	request := verifiedAnonymousRenewRequest("operation:renew:concurrent")
	seedExpiredAnonymousAccess(t, ctx, database, request, false)
	manager, accessor := newVerifiedAnonymousTokenStubs(t)
	service, err := NewVerifiedAnonymousCreator(database, manager, accessor)
	if err != nil {
		t.Fatal(err)
	}
	const callers = 8
	results := make([]*supportcap.RenewAnonymousPresalesAccessResult, callers)
	errorsByCaller := make([]error, callers)
	var wait sync.WaitGroup
	for index := range callers {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			results[index], errorsByCaller[index] = service.RenewAnonymousPresalesAccess(context.Background(), request)
		}(index)
	}
	wait.Wait()
	for index := range callers {
		if errorsByCaller[index] != nil || results[index] == nil {
			t.Fatalf("concurrent renew caller %d result=%+v err=%v", index, results[index], errorsByCaller[index])
		}
		if results[index].AccessToken != results[0].AccessToken ||
			!results[index].ExpiresAt.Equal(results[0].ExpiresAt) {
			t.Fatalf("concurrent renew caller %d returned a different result", index)
		}
	}
	assertVerifiedAnonymousRenewalRows(t, ctx, database, request, results[0])
}

func verifiedAnonymousRequest(operationNo string) supportcap.CreateAnonymousPresalesRequest {
	return supportcap.CreateAnonymousPresalesRequest{
		DestinationKind: supportcap.DestinationEmail, DestinationDigest: strings.Repeat("a", 64),
		CategoryNo: "CAT_public", Subject: "Need a server", Body: "Please contact me", OperationNo: operationNo,
	}
}

func verifiedAnonymousRenewRequest(operationNo string) supportcap.RenewAnonymousPresalesAccessRequest {
	return supportcap.RenewAnonymousPresalesAccessRequest{
		DestinationKind: supportcap.DestinationSMS, DestinationDigest: strings.Repeat("b", 64),
		TicketNo: "TKT_renew_anonymous", OperationNo: operationNo,
	}
}

func seedExpiredAnonymousAccess(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	request supportcap.RenewAnonymousPresalesAccessRequest,
	active bool,
) {
	t.Helper()
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket (tenant_id,ticket_no,ticket_type,visibility_code,anonymous_destination_digest,anonymous_destination_kind,subject,base_stage,priority_code,current_round,last_activity_at,created_by,updated_by) VALUES (0,?,'presales','anonymous',?,?,'Existing anonymous ticket','in_progress','normal',1,CURRENT_TIMESTAMP,0,0)`, request.TicketNo, request.DestinationDigest, request.DestinationKind); err != nil {
		t.Fatal(err)
	}
	createdOffset, expiryOffset := "-3 hours", "-1 hour"
	if active {
		createdOffset, expiryOffset = "-1 hour", "+1 hour"
	}
	query := `INSERT INTO plugin_idc_core_support_ticket_access_link (tenant_id,link_no,ticket_no,token_digest,destination_digest,destination_kind,expires_at,created_at) VALUES (0,'SAL_expired_original',?, ?,?,?,CURRENT_TIMESTAMP+INTERVAL '` + expiryOffset + `',CURRENT_TIMESTAMP+INTERVAL '` + createdOffset + `')`
	if _, err := database.Exec(ctx, query, request.TicketNo, anonymousTokenDigest("old-anonymous-token"), request.DestinationDigest, request.DestinationKind); err != nil {
		t.Fatal(err)
	}
}

func assertVerifiedAnonymousRenewalRows(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	request supportcap.RenewAnonymousPresalesAccessRequest,
	result *supportcap.RenewAnonymousPresalesAccessResult,
) {
	t.Helper()
	operationDigest, err := supportcap.RenewOperationDigest(request.OperationNo)
	if err != nil {
		t.Fatal(err)
	}
	var row struct {
		TicketNo, OperationDigest, RequestDigest, TokenDigest, TokenSecretRef string
		DestinationDigest, DestinationKind                                    string
		CreatedAt, ExpiresAt                                                  time.Time
	}
	if err = database.GetScan(ctx, &row, `SELECT ticket_no,operation_digest,request_digest,token_digest,token_secret_ref,destination_digest,destination_kind,created_at,expires_at FROM plugin_idc_core_support_ticket_access_link WHERE operation_digest=?`, operationDigest); err != nil {
		t.Fatal(err)
	}
	if row.TicketNo != request.TicketNo || row.OperationDigest != operationDigest ||
		row.OperationDigest == request.OperationNo || row.RequestDigest == "" ||
		row.TokenDigest != anonymousTokenDigest(result.AccessToken) || row.TokenSecretRef != "SEC_verified_anonymous_test" ||
		row.DestinationDigest != request.DestinationDigest || row.DestinationKind != string(request.DestinationKind) ||
		!row.ExpiresAt.Equal(row.CreatedAt.Add(2*time.Hour)) || !row.ExpiresAt.Equal(result.ExpiresAt) {
		t.Fatalf("unexpected renewed anonymous link: %+v", row)
	}
	oldRevoked, err := queryCount(ctx, database, `SELECT COUNT(1) FROM plugin_idc_core_support_ticket_access_link WHERE link_no='SAL_expired_original' AND revoked_at IS NOT NULL`)
	if err != nil || oldRevoked != 1 {
		t.Fatalf("old link was not permanently revoked: count=%d err=%v", oldRevoked, err)
	}
	links, err := queryCount(ctx, database, `SELECT COUNT(1) FROM plugin_idc_core_support_ticket_access_link`)
	if err != nil || links != 2 {
		t.Fatalf("renewal links=%d want=2 err=%v", links, err)
	}
}

func seedVerifiedAnonymousRoute(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_support_department (tenant_id,department_no,name_zh_cn,name_en_us,assignment_mode,status_code,version,created_by,updated_by) VALUES (0,'DEP_sales','售前','Presales','manual_claim','active',1,0,0)`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_support_issue_category (tenant_id,category_no,name_zh_cn,name_en_us,department_no,priority_code,sort_order,status_code,version,created_by,updated_by) VALUES (0,'CAT_public','售前咨询','Presales','DEP_sales','high',10,'active',1,0,0)`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_support_sla_calendar (tenant_id,calendar_no,name,mode_code,status_code,version,created_by,updated_by) VALUES (0,'SLC_sales','全天','continuous','active',1,0,0)`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_support_sla_rule (tenant_id,rule_no,department_no,priority_code,calendar_no,first_response_minutes,resolution_minutes,near_due_minutes,status_code,version,created_by,updated_by) VALUES (0,'SLR_sales','DEP_sales','high','SLC_sales',30,240,ARRAY[15],'active',1,0,0)`); err != nil {
		t.Fatal(err)
	}
}

func assertVerifiedAnonymousRows(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	request supportcap.CreateAnonymousPresalesRequest,
	result *supportcap.CreateAnonymousPresalesResult,
) {
	t.Helper()
	var row struct {
		TicketNo, TicketType, Visibility, DestinationDigest, DestinationKind string
		DepartmentNo, Priority, Stage, Subject                               string
		OperationDigest, RequestDigest, TokenDigest, TokenSecretRef          string
		Body                                                                 string
		CreatedAt, ExpiresAt                                                 time.Time
	}
	if err := database.GetScan(ctx, &row, `SELECT t.ticket_no,t.ticket_type,t.visibility_code AS visibility,t.anonymous_destination_digest AS destination_digest,t.anonymous_destination_kind AS destination_kind,COALESCE(t.department_no,'') AS department_no,t.priority_code AS priority,t.base_stage AS stage,t.subject,m.body,l.operation_digest,l.request_digest,l.token_digest,l.token_secret_ref,l.created_at,l.expires_at FROM plugin_idc_core_support_ticket t JOIN plugin_idc_core_support_ticket_message m ON m.ticket_no=t.ticket_no AND m.message_type='initial' JOIN plugin_idc_core_support_ticket_access_link l ON l.ticket_no=t.ticket_no WHERE t.ticket_no=?`, result.TicketNo); err != nil {
		t.Fatal(err)
	}
	if row.TicketType != "presales" || row.Visibility != "anonymous" ||
		row.DestinationDigest != request.DestinationDigest || row.DestinationKind != string(request.DestinationKind) ||
		row.DepartmentNo != "DEP_sales" || row.Priority != "high" || row.Stage != "new" ||
		row.Subject != request.Subject || row.Body != request.Body || row.TokenSecretRef != "SEC_verified_anonymous_test" ||
		row.TokenDigest == verifiedAnonymousTestToken || row.OperationDigest == request.OperationNo ||
		row.RequestDigest == "" || !row.ExpiresAt.Equal(row.CreatedAt.Add(2*time.Hour)) ||
		!row.ExpiresAt.Equal(result.ExpiresAt) {
		t.Fatalf("unexpected persisted anonymous support row: %+v", row)
	}
	for table, query := range map[string]string{
		"ticket":     `SELECT COUNT(1) FROM plugin_idc_core_support_ticket`,
		"message":    `SELECT COUNT(1) FROM plugin_idc_core_support_ticket_message`,
		"status":     `SELECT COUNT(1) FROM plugin_idc_core_support_ticket_status_history`,
		"assignment": `SELECT COUNT(1) FROM plugin_idc_core_support_ticket_assignment_history`,
		"sla":        `SELECT COUNT(1) FROM plugin_idc_core_support_ticket_sla_round`,
		"link":       `SELECT COUNT(1) FROM plugin_idc_core_support_ticket_access_link`,
	} {
		count, err := queryCount(ctx, database, query)
		if err != nil || count != 1 {
			t.Fatalf("%s rows=%d err=%v", table, count, err)
		}
	}
}
