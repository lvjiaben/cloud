// This file exercises custom-field snapshots, service ownership, split inheritance, and data visibility against PostgreSQL.
package supportdesk

import (
	"context"
	"crypto/rand"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

func TestCustomFieldsAndEntitlementAssociationPostgreSQL(t *testing.T) {
	ctx, database := newSupportDeskTestDatabase(t)
	staff := newAuthorizedSupportDeskTestService(database)
	customer := newCustomerSupportDeskTestService(database, "CUS_owner")
	seedCustomFieldCustomerFacts(t, ctx, database)

	definitions := make([]*CustomFieldDefinition, 0, 7)
	for index, fieldType := range []string{
		customFieldSingleLine, customFieldMultiLine, customFieldNumber, customFieldSingleSelect,
		customFieldMultiSelect, customFieldDate, customFieldBoolean,
	} {
		input := CustomFieldSettingInput{
			FieldType: fieldType, NameZhCN: "中文字段", NameEnUS: "English field",
			DescriptionZhCN: "中文说明", DescriptionEnUS: "English description",
			AppliesCustomer: true, Visibility: customFieldCustomer, Required: true,
			SortOrder: index + 1, Status: SettingStatusActive,
		}
		if fieldType == customFieldSingleSelect || fieldType == customFieldMultiSelect {
			input.Options = []CustomFieldOptionInput{
				{NameZhCN: "选项一", NameEnUS: "Option one", SortOrder: 1, Status: SettingStatusActive},
				{NameZhCN: "选项二", NameEnUS: "Option two", SortOrder: 2, Status: SettingStatusActive},
			}
		}
		item, err := staff.CreateCustomFieldSetting(ctx, input)
		if err != nil {
			t.Fatalf("create %s custom field: %v", fieldType, err)
		}
		definitions = append(definitions, item)
	}

	text, multi, number, date, boolean := "  request value  ", "line one\nline two", "00012.3400", "2026-08-15", false
	submissions := []CustomFieldSubmission{
		{FieldNo: definitions[0].FieldNo, TextValue: &text},
		{FieldNo: definitions[1].FieldNo, TextValue: &multi},
		{FieldNo: definitions[2].FieldNo, NumberValue: &number},
		{FieldNo: definitions[3].FieldNo, OptionNos: []string{definitions[3].Options[0].OptionNo}},
		{FieldNo: definitions[4].FieldNo, OptionNos: []string{definitions[4].Options[1].OptionNo, definitions[4].Options[0].OptionNo}},
		{FieldNo: definitions[5].FieldNo, DateValue: &date},
		{FieldNo: definitions[6].FieldNo, BooleanValue: &boolean},
	}
	detail, err := customer.CreateAccount(ctx, CreateInput{
		CategoryNo: "CAT_support", Subject: "Entitlement issue", Body: "Please investigate",
		AssociationReference: supportAssociationReference(t, ctx, database, "ent_active"), CustomFields: submissions,
	})
	if err != nil {
		t.Fatalf("create customer ticket: %v", err)
	}
	if detail.Ticket.RelatedEntitlementNo != "ent_active" || detail.Ticket.RelatedEntitlementVersion != 3 ||
		detail.Ticket.RelatedEntitlementNameZhCN != "手机套餐" || !detail.Ticket.CurrentEntitlementAvailable {
		t.Fatalf("unexpected entitlement snapshot/current projection: %+v", detail.Ticket)
	}
	if len(detail.CustomFields) != 7 || detail.CustomFields[2].NormalizedValue != "12.34" ||
		detail.CustomFields[6].NormalizedValue != "false" || len(detail.CustomFields[4].SelectedOptions) != 2 {
		t.Fatalf("unexpected custom field snapshots: %+v", detail.CustomFields)
	}
	firstCandidates, err := customer.SearchAccountEntitlementCandidates(ctx, "", "", "", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(firstCandidates.Items) != 2 || firstCandidates.NextCursor == "" {
		t.Fatalf("first bounded entitlement page is invalid: %+v", firstCandidates)
	}
	secondCandidates, err := customer.SearchAccountEntitlementCandidates(ctx, "", "", firstCandidates.NextCursor, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(secondCandidates.Items) != 1 || secondCandidates.NextCursor != "" ||
		secondCandidates.Items[0].EntitlementNo == firstCandidates.Items[0].EntitlementNo {
		t.Fatalf("second bounded entitlement page is invalid: %+v", secondCandidates)
	}

	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_catalog_product SET name_zh_cn='手机套餐新名称' WHERE product_no='PRD_phone'`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_service_entitlement SET status_code='terminated',version=4 WHERE entitlement_no='ent_active'`); err != nil {
		t.Fatal(err)
	}
	detail, err = customer.DetailAccount(ctx, detail.Ticket.TicketNo)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Ticket.RelatedEntitlementNameZhCN != "手机套餐" || detail.Ticket.RelatedEntitlementStatus != "active" ||
		detail.Ticket.CurrentEntitlementNameZhCN != "手机套餐" || detail.Ticket.CurrentEntitlementStatus != "terminated" || detail.Ticket.CurrentEntitlementVersion != 4 {
		t.Fatalf("snapshot changed or current projection stale: %+v", detail.Ticket)
	}

	disabledInput := customFieldInputFromDefinition(definitions[1])
	disabledInput.NameZhCN = "已停用的新名称"
	disabledInput.NameEnUS = "Disabled new name"
	disabledInput.Status = SettingStatusDisabled
	if _, err = staff.UpdateCustomFieldSetting(ctx, definitions[1].FieldNo, disabledInput); err != nil {
		t.Fatalf("disable referenced definition: %v", err)
	}
	optionRename := customFieldInputFromDefinition(definitions[3])
	optionRename.Options[0].NameZhCN = "新选项名称"
	optionRename.Options[0].NameEnUS = "Renamed option"
	if _, err = staff.UpdateCustomFieldSetting(ctx, definitions[3].FieldNo, optionRename); err != nil {
		t.Fatalf("rename referenced option: %v", err)
	}
	detail, err = customer.DetailAccount(ctx, detail.Ticket.TicketNo)
	if err != nil {
		t.Fatal(err)
	}
	if detail.CustomFields[1].NameZhCN != "中文字段" ||
		detail.CustomFields[3].SelectedOptions[0].NameZhCN != "选项一" {
		t.Fatalf("referenced definition or option rewrote immutable snapshots: %+v", detail.CustomFields)
	}
	activeDefinitions, err := customer.AccountCustomFields(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(activeDefinitions) != 6 {
		t.Fatalf("disabled definition remained in customer directory: %+v", activeDefinitions)
	}

	sourceTicketNo := detail.Ticket.TicketNo
	splitDetail, err := staff.Split(ctx, SplitInput{
		TicketNo: sourceTicketNo, TicketVersion: detail.Ticket.Version,
		CategoryNo: "CAT_support", Reason: "Separate the second issue",
	})
	if err != nil {
		t.Fatalf("split ticket with historical snapshots: %v", err)
	}
	if splitDetail.Ticket.TicketNo == sourceTicketNo ||
		splitDetail.Ticket.RelatedEntitlementNo != "ent_active" ||
		splitDetail.Ticket.RelatedEntitlementNameZhCN != "手机套餐" ||
		splitDetail.Ticket.RelatedEntitlementStatus != "active" ||
		splitDetail.Ticket.CurrentEntitlementStatus != "terminated" ||
		len(splitDetail.CustomFields) != 7 ||
		splitDetail.CustomFields[1].NameZhCN != "中文字段" ||
		splitDetail.CustomFields[3].SelectedOptions[0].NameZhCN != "选项一" {
		t.Fatalf("split did not inherit immutable snapshots with current projection: %+v fields=%+v", splitDetail.Ticket, splitDetail.CustomFields)
	}
	valueCount, err := queryCount(ctx, database, `SELECT COUNT(1) FROM plugin_idc_core_support_ticket_custom_field_value WHERE tenant_id=0 AND ticket_no IN (?,?)`, sourceTicketNo, splitDetail.Ticket.TicketNo)
	if err != nil || valueCount != 14 {
		t.Fatalf("split custom-field snapshot rows=%d err=%v", valueCount, err)
	}
	optionValueCount, err := queryCount(ctx, database, `SELECT COUNT(1) FROM plugin_idc_core_support_ticket_custom_field_value_option WHERE tenant_id=0 AND ticket_no IN (?,?)`, sourceTicketNo, splitDetail.Ticket.TicketNo)
	if err != nil || optionValueCount != 6 {
		t.Fatalf("split custom-field option snapshot rows=%d err=%v", optionValueCount, err)
	}
	sourceAfterSplit, err := customer.DetailAccount(ctx, sourceTicketNo)
	if err != nil {
		t.Fatal(err)
	}
	if sourceAfterSplit.Ticket.BaseStage != "new" || sourceAfterSplit.Ticket.RelatedEntitlementNo != "ent_active" ||
		len(sourceAfterSplit.CustomFields) != 7 {
		t.Fatalf("split changed source ticket business facts: %+v fields=%+v", sourceAfterSplit.Ticket, sourceAfterSplit.CustomFields)
	}
	pageAfterSplit, err := customer.SearchAccount(ctx, SearchInput{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(pageAfterSplit.Tickets) != 2 {
		t.Fatalf("customer ticket page after split=%d want=2", len(pageAfterSplit.Tickets))
	}
	for _, ticket := range pageAfterSplit.Tickets {
		if ticket.RelatedEntitlementNo != "ent_active" || ticket.CurrentEntitlementStatus != "terminated" ||
			!ticket.CurrentEntitlementAvailable {
			t.Fatalf("list batch projection lost inherited current entitlement state: %+v", ticket)
		}
	}

	changeType := customFieldInputFromDefinition(definitions[0])
	changeType.FieldType = customFieldMultiLine
	if _, err = staff.UpdateCustomFieldSetting(ctx, definitions[0].FieldNo, changeType); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("referenced field type change err=%v, want conflict", err)
	}
	if _, err = database.Exec(ctx, `DELETE FROM plugin_idc_core_support_custom_field_definition WHERE field_no=?`, definitions[0].FieldNo); err == nil {
		t.Fatal("referenced custom field definition was physically deleted")
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_support_ticket_custom_field_value SET normalized_value='changed' WHERE ticket_no=? AND field_no=?`, detail.Ticket.TicketNo, definitions[0].FieldNo); err == nil {
		t.Fatal("immutable custom field snapshot was updated")
	}
	if _, err = database.Exec(ctx, `DELETE FROM plugin_idc_core_support_custom_field_option WHERE option_no=?`, definitions[3].Options[0].OptionNo); err == nil {
		t.Fatal("referenced custom field option was physically deleted")
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_support_ticket SET related_entitlement_no='ent_pending',related_entitlement_version=1,related_entitlement_name_zh_cn='手机套餐',related_entitlement_name_en_us='Phone plan',related_entitlement_status='suspended' WHERE ticket_no=?`, detail.Ticket.TicketNo); err == nil {
		t.Fatal("immutable ticket entitlement relation was changed")
	}

	candidates, err := customer.SearchAccountEntitlementCandidates(ctx, "", "", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates.Items) != 2 || candidates.Items[0].EntitlementNo != "ent_expired" {
		t.Fatalf("unsafe or terminal entitlement candidates returned: %+v", candidates.Items)
	}
	ownedReference := supportAssociationReference(t, ctx, database, "ent_pending")
	ownedCandidates, err := customer.SearchAccountEntitlementCandidates(ctx, "", ownedReference, "", 10)
	if err != nil || len(ownedCandidates.Items) != 1 || ownedCandidates.Items[0].AssociationReference != ownedReference {
		t.Fatalf("owned association lookup candidates=%+v err=%v", ownedCandidates, err)
	}
	otherCandidates, err := customer.SearchAccountEntitlementCandidates(ctx, "", supportAssociationReference(t, ctx, database, "ent_other"), "", 10)
	if err != nil || len(otherCandidates.Items) != 0 {
		t.Fatalf("cross-customer association lookup candidates=%+v err=%v", otherCandidates, err)
	}
	for _, entitlementNo := range []string{"ent_other", "ent_terminated"} {
		associationReference := supportAssociationReference(t, ctx, database, entitlementNo)
		if _, err = customer.CreateAccount(ctx, CreateInput{CategoryNo: "CAT_support", Subject: "Unsafe entitlement", Body: "Must fail", AssociationReference: associationReference, CustomFields: submissions}); !bizerr.Is(err, CodeInvalid) {
			t.Fatalf("association for %s create err=%v, want invalid", entitlementNo, err)
		}
	}
	if _, err = customer.CreateAccount(ctx, CreateInput{CategoryNo: "CAT_support", Subject: "Missing entitlement", Body: "Must fail", AssociationReference: "sar_00000000000000000000000000000000", CustomFields: submissions}); !bizerr.Is(err, CodeInvalid) {
		t.Fatalf("missing association create err=%v, want same invalid result", err)
	}
	if _, err = customer.CreateAccount(ctx, CreateInput{CategoryNo: "CAT_support", Subject: "Missing fields", Body: "Must fail"}); !bizerr.Is(err, CodeInvalid) {
		t.Fatalf("missing required custom fields err=%v, want invalid", err)
	}
	if _, err = staff.CreateStaff(ctx, CreateInput{DepartmentNo: "DEP_internal", Subject: "Internal", Body: "Must fail", EntitlementNo: "ent_pending"}); !bizerr.Is(err, CodeInvalid) {
		t.Fatalf("internal ticket entitlement relation err=%v, want invalid", err)
	}
	created, countErr := queryCount(ctx, database, `SELECT COUNT(1) FROM plugin_idc_core_support_ticket`)
	if countErr != nil || created != 2 {
		t.Fatalf("invalid creates wrote tickets: count=%d err=%v", created, countErr)
	}
}

func TestCustomerCustomFieldReadNeverReturnsInternalSnapshotPostgreSQL(t *testing.T) {
	ctx, database := newSupportDeskTestDatabase(t)
	seedCustomFieldCustomerFacts(t, ctx, database)
	customer := newCustomerSupportDeskTestService(database, "CUS_owner")
	detail, err := customer.CreateAccount(ctx, CreateInput{CategoryNo: "CAT_support", Subject: "No fields", Body: "Public body"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_support_custom_field_definition (tenant_id,field_no,field_type,name_zh_cn,name_en_us,applies_internal,visibility_code,required,sort_order,status_code,version) VALUES (0,'SCF_internal_leak','single_line','内部','Internal',TRUE,'internal_only',FALSE,1,'active',1)`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket_custom_field_value (tenant_id,ticket_no,field_no,definition_version,field_type,name_zh_cn,name_en_us,visibility_code,required,sort_order,normalized_value,selected_option_count) VALUES (0,?,'SCF_internal_leak',1,'single_line','内部','Internal','internal_only',FALSE,1,'secret',0)`, detail.Ticket.TicketNo); err != nil {
		t.Fatal(err)
	}
	detail, err = customer.DetailAccount(ctx, detail.Ticket.TicketNo)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.CustomFields) != 0 {
		t.Fatalf("customer detail leaked internal custom fields: %+v", detail.CustomFields)
	}
}

type customerSupportDeskBizCtxStub struct{ userID int }

func (s customerSupportDeskBizCtxStub) Current(context.Context) bizctxcap.CurrentContext {
	return bizctxcap.CurrentContext{UserID: s.userID, TenantID: 0}
}

type customerSupportDeskPrincipalStub struct {
	userID     int
	customerNo string
}

func (s customerSupportDeskPrincipalStub) Current(context.Context) (*principal.Audience, error) {
	return &principal.Audience{UserID: s.userID, Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive, CustomerNo: s.customerNo, Version: 1}, nil
}

func newCustomerSupportDeskTestService(database gdb.DB, customerNo string) *serviceImpl {
	const userID = 8
	return &serviceImpl{database: database, bizCtx: customerSupportDeskBizCtxStub{userID: userID}, principal: customerSupportDeskPrincipalStub{userID: userID, customerNo: customerNo}, random: rand.Reader}
}

func seedCustomFieldCustomerFacts(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	statements := []string{
		`INSERT INTO plugin_idc_core_customer_subject (tenant_id,customer_no,display_name) VALUES (0,'CUS_owner','Owner'),(0,'CUS_other','Other')`,
		`INSERT INTO plugin_idc_core_support_issue_category (tenant_id,category_no,name_zh_cn,name_en_us,priority_code,status_code) VALUES (0,'CAT_support','技术支持','Support','normal','active')`,
		`INSERT INTO plugin_idc_core_support_department (tenant_id,department_no,name_zh_cn,name_en_us,assignment_mode,status_code) VALUES (0,'DEP_internal','内部','Internal','manual_claim','active')`,
		`INSERT INTO plugin_idc_core_catalog_product (tenant_id,product_no,name_zh_cn,name_en_us) VALUES (0,'PRD_phone','手机套餐','Phone plan')`,
		`INSERT INTO plugin_idc_core_product_fulfillment_intent (tenant_id,intent_no,product_name_zh_cn,product_name_en_us) VALUES
			(0,'INT_active','手机套餐','Phone plan'),(0,'INT_suspended','手机套餐','Phone plan'),
			(0,'INT_terminated','手机套餐','Phone plan'),(0,'INT_expired','手机套餐','Phone plan'),
			(0,'INT_other','手机套餐','Phone plan')`,
		`INSERT INTO plugin_idc_core_service_entitlement (tenant_id,entitlement_no,source_intent_no,customer_no,status_code,version) VALUES
			(0,'ent_active','INT_active','CUS_owner','active',3),
			(0,'ent_pending','INT_suspended','CUS_owner','suspended',1),
			(0,'ent_terminated','INT_terminated','CUS_owner','terminated',2),
			(0,'ent_expired','INT_expired','CUS_owner','expired',2),
			(0,'ent_other','INT_other','CUS_other','active',1)`,
	}
	for _, statement := range statements {
		if _, err := database.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
}

func supportAssociationReference(t *testing.T, ctx context.Context, database gdb.DB, entitlementNo string) string {
	t.Helper()
	var rows []struct {
		Reference string `orm:"support_association_reference"`
	}
	if err := database.GetScan(ctx, &rows, `SELECT support_association_reference FROM plugin_idc_core_service_entitlement WHERE tenant_id=0 AND entitlement_no=?`, entitlementNo); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Reference == "" {
		t.Fatalf("support association reference for %s is empty", entitlementNo)
	}
	return rows[0].Reference
}

func customFieldInputFromDefinition(item *CustomFieldDefinition) CustomFieldSettingInput {
	options := make([]CustomFieldOptionInput, 0, len(item.Options))
	for _, option := range item.Options {
		options = append(options, CustomFieldOptionInput{OptionNo: option.OptionNo, NameZhCN: option.NameZhCN, NameEnUS: option.NameEnUS, SortOrder: option.SortOrder, Status: option.Status, ExpectedVersion: option.Version})
	}
	return CustomFieldSettingInput{FieldType: item.FieldType, NameZhCN: item.NameZhCN, NameEnUS: item.NameEnUS, DescriptionZhCN: item.DescriptionZhCN, DescriptionEnUS: item.DescriptionEnUS, AppliesPresales: item.AppliesPresales, AppliesCustomer: item.AppliesCustomer, AppliesInternal: item.AppliesInternal, Visibility: item.Visibility, Required: item.Required, SortOrder: item.SortOrder, Status: item.Status, ExpectedVersion: item.Version, Options: options}
}
