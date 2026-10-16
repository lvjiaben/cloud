package supportdesk

import (
	"sync"
	"testing"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"

	"lina-core/pkg/bizerr"
)

func TestSupportSettingsLifecyclePostgreSQL(t *testing.T) {
	ctx, database := newSupportDeskTestDatabase(t)
	service := newAuthorizedSupportDeskTestService(database)
	service.organization = &queueOrgStub{deptID: 11}

	department, err := service.CreateDepartmentSetting(ctx, DepartmentSettingInput{OrgDeptID: 11, NameZhCN: "客户支持", NameEnUS: "Customer Support"})
	if err != nil {
		t.Fatal(err)
	}
	if department.AssignmentMode != string(AssignmentModeManualClaim) || department.Status != string(SettingStatusActive) || department.Version != 1 {
		t.Fatalf("unexpected department defaults: %+v", department)
	}
	category, err := service.CreateIssueCategorySetting(ctx, IssueCategorySettingInput{NameZhCN: "网络问题", NameEnUS: "Network", DepartmentNo: department.DepartmentNo, Priority: "high", SortOrder: 10})
	if err != nil {
		t.Fatal(err)
	}
	status, err := service.CreateCustomStatusSetting(ctx, CustomStatusSettingInput{NameZhCN: "待确认", NameEnUS: "Confirming", BaseStage: "waiting_customer", SortOrder: 20})
	if err != nil {
		t.Fatal(err)
	}
	if status.BaseStage != "waiting_customer" || status.Status != SettingStatusActive {
		t.Fatalf("unexpected custom status: %+v", status)
	}
	preset, err := service.CreateReplyPresetSetting(ctx, ReplyPresetSettingInput{DepartmentNo: department.DepartmentNo, Locale: "zh-CN", Title: "收到", Body: "我们已经收到您的问题。", SortOrder: 1})
	if err != nil {
		t.Fatal(err)
	}
	if preset.DepartmentNo != department.DepartmentNo || preset.Locale != "zh-CN" {
		t.Fatalf("unexpected preset: %+v", preset)
	}
	start, end := 9*60, 18*60
	calendar, err := service.CreateSLACalendarSetting(ctx, SLACalendarSettingInput{Name: "工作日", Mode: "business_hours", Timezone: "Asia/Shanghai", Workdays: []int{5, 1, 3, 1, 2, 4}, WorkStartMinute: &start, WorkEndMinute: &end})
	if err != nil {
		t.Fatal(err)
	}
	if got := calendar.Workdays; len(got) != 5 || got[0] != 1 || got[4] != 5 {
		t.Fatalf("calendar workdays not normalized: %v", got)
	}
	holiday, err := service.PutSLAHolidaySetting(ctx, calendar.CalendarNo, SLAHolidaySettingInput{Date: "2026-10-01", Name: "国庆节"})
	if err != nil {
		t.Fatal(err)
	}
	if holiday.Date != "2026-10-01" {
		t.Fatalf("unexpected holiday: %+v", holiday)
	}
	firstResponse, resolution := 30, 240
	rule, err := service.CreateSLARuleSetting(ctx, SLARuleSettingInput{DepartmentNo: department.DepartmentNo, CategoryNo: category.CategoryNo, Priority: "high", CalendarNo: calendar.CalendarNo, FirstResponseMinutes: &firstResponse, ResolutionMinutes: &resolution, NearDueMinutes: []int{15, 60, 15}})
	if err != nil {
		t.Fatal(err)
	}
	if len(rule.NearDueMinutes) != 2 || rule.NearDueMinutes[0] != 60 || rule.NearDueMinutes[1] != 15 {
		t.Fatalf("reminder nodes not canonical: %v", rule.NearDueMinutes)
	}

	if _, err = service.UpdateDepartmentSetting(ctx, department.DepartmentNo, DepartmentSettingInput{OrgDeptID: 11, NameZhCN: "Support", NameEnUS: "Support", AssignmentMode: AssignmentModeLeastUnresolved, ExpectedVersion: department.Version}); !bizerr.Is(err, CodeInvalid) {
		t.Fatalf("legacy automatic assignment accepted=%v", err)
	}
	updated, err := service.UpdateDepartmentSetting(ctx, department.DepartmentNo, DepartmentSettingInput{OrgDeptID: 11, NameZhCN: "技术支持", NameEnUS: "Technical Support", AssignmentMode: AssignmentModeManualClaim, Status: SettingStatusActive, ExpectedVersion: department.Version})
	if err != nil {
		t.Fatal(err)
	}
	if _, calendarErr := service.UpdateSLACalendarSetting(ctx, calendar.CalendarNo, SLACalendarSettingInput{Name: calendar.Name, Mode: calendar.Mode, Timezone: calendar.Timezone, Workdays: calendar.Workdays, WorkStartMinute: calendar.WorkStartMinute, WorkEndMinute: calendar.WorkEndMinute, Status: SettingStatusDisabled, ExpectedVersion: calendar.Version}); !bizerr.Is(calendarErr, CodeConflict) {
		t.Fatalf("referenced calendar update-disable error=%v, want conflict", calendarErr)
	}
	if _, calendarErr := service.DisableSLACalendarSetting(ctx, calendar.CalendarNo, calendar.Version); !bizerr.Is(calendarErr, CodeConflict) {
		t.Fatalf("referenced calendar disable error=%v, want conflict", calendarErr)
	}
	if updated.Version != 2 || updated.AssignmentMode != string(AssignmentModeManualClaim) {
		t.Fatalf("unexpected department update: %+v", updated)
	}
	if _, err = service.UpdateDepartmentSetting(ctx, department.DepartmentNo, DepartmentSettingInput{OrgDeptID: 11, NameZhCN: "过期写入", NameEnUS: "Stale", AssignmentMode: AssignmentModeManualClaim, Status: SettingStatusActive, ExpectedVersion: department.Version}); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("stale update error=%v, want conflict", err)
	}
	if _, err = service.DisableDepartmentSetting(ctx, department.DepartmentNo, updated.Version); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("referenced department disable error=%v, want conflict", err)
	}

	if _, err = service.DisableSLARuleSetting(ctx, rule.RuleNo, rule.Version); err != nil {
		t.Fatal(err)
	}
	if _, err = service.DisableIssueCategorySetting(ctx, category.CategoryNo, category.Version); err != nil {
		t.Fatal(err)
	}
	disabledDepartment, err := service.DisableDepartmentSetting(ctx, department.DepartmentNo, updated.Version)
	if err != nil {
		t.Fatal(err)
	}
	if disabledDepartment.Status != string(SettingStatusDisabled) {
		t.Fatalf("department was not disabled: %+v", disabledDepartment)
	}
	if err = service.DeleteSLAHolidaySetting(ctx, calendar.CalendarNo, holiday.Date); err != nil {
		t.Fatal(err)
	}
	holidays, err := service.SLAHolidaySettings(ctx, calendar.CalendarNo)
	if err != nil || len(holidays) != 0 {
		t.Fatalf("holidays after delete=%+v err=%v", holidays, err)
	}
}

func TestSupportSLARuleScopeIsUniqueUnderConcurrencyPostgreSQL(t *testing.T) {
	ctx, database := newSupportDeskTestDatabase(t)
	service := newAuthorizedSupportDeskTestService(database)
	service.organization = &queueOrgStub{deptID: 11}
	department, err := service.CreateDepartmentSetting(ctx, DepartmentSettingInput{OrgDeptID: 11, NameZhCN: "支持", NameEnUS: "Support"})
	if err != nil {
		t.Fatal(err)
	}
	calendar, err := service.CreateSLACalendarSetting(ctx, SLACalendarSettingInput{Name: "全天", Mode: "continuous"})
	if err != nil {
		t.Fatal(err)
	}
	firstResponse := 60
	input := SLARuleSettingInput{DepartmentNo: department.DepartmentNo, Priority: "normal", CalendarNo: calendar.CalendarNo, FirstResponseMinutes: &firstResponse}

	var wait sync.WaitGroup
	errors := make(chan error, 2)
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, createErr := service.CreateSLARuleSetting(ctx, input)
			errors <- createErr
		}()
	}
	wait.Wait()
	close(errors)
	var succeeded, conflicted int
	for createErr := range errors {
		switch {
		case createErr == nil:
			succeeded++
		case bizerr.Is(createErr, CodeConflict):
			conflicted++
		default:
			t.Fatalf("unexpected concurrent create error: %v", createErr)
		}
	}
	if succeeded != 1 || conflicted != 1 {
		t.Fatalf("concurrent SLA rule results success/conflict=%d/%d", succeeded, conflicted)
	}
}

func TestUpstreamMappingsRequireTheLatestVersionPostgreSQL(t *testing.T) {
	ctx, database := newSupportDeskTestDatabase(t)
	service := newAuthorizedSupportDeskTestService(database)
	service.organization = &queueOrgStub{deptID: 11}
	department, err := service.CreateDepartmentSetting(ctx, DepartmentSettingInput{OrgDeptID: 11, NameZhCN: "支持", NameEnUS: "Support"})
	if err != nil {
		t.Fatal(err)
	}

	departmentMap, err := service.SaveUpstreamDepartmentMap(ctx, UpstreamDepartmentMapInput{
		DepartmentNo: department.DepartmentNo, UpstreamDepartmentID: 12,
		UpstreamDepartmentName: "Technical support", Status: "active", ExpectedVersion: 0,
	})
	if err != nil || departmentMap.Version != 1 {
		t.Fatalf("create department map = %+v, %v", departmentMap, err)
	}
	if _, err = service.SaveUpstreamDepartmentMap(ctx, UpstreamDepartmentMapInput{
		DepartmentNo: department.DepartmentNo, UpstreamDepartmentID: 13,
		UpstreamDepartmentName: "Stale", Status: "active", ExpectedVersion: 0,
	}); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("zero-version overwrite error = %v, want conflict", err)
	}
	departmentMap, err = service.SaveUpstreamDepartmentMap(ctx, UpstreamDepartmentMapInput{
		DepartmentNo: department.DepartmentNo, UpstreamDepartmentID: 13,
		UpstreamDepartmentName: "Updated", Status: "active", ExpectedVersion: departmentMap.Version,
	})
	if err != nil || departmentMap.Version != 2 || departmentMap.UpstreamDepartmentID != 13 {
		t.Fatalf("update department map = %+v, %v", departmentMap, err)
	}

	if _, err = service.database.Exec(ctx, `INSERT INTO plugin_idc_core_service_entitlement(tenant_id,entitlement_no,source_intent_no,customer_no,status_code,version) VALUES(0,'ent_support_01','INT_support_01','CUS_support','active',1)`); err != nil {
		t.Fatal(err)
	}
	entitlementMap, err := service.SaveUpstreamEntitlementMap(ctx, UpstreamEntitlementMapInput{
		EntitlementNo: "ent_support_01", UpstreamReference: "opaque_8086", Status: "active", ExpectedVersion: 0,
	})
	if err != nil || entitlementMap.Version != 1 {
		t.Fatalf("create entitlement map = %+v, %v", entitlementMap, err)
	}
	if _, err = service.SaveUpstreamEntitlementMap(ctx, UpstreamEntitlementMapInput{
		EntitlementNo: "ent_support_01", UpstreamReference: "opaque_8087", Status: "active", ExpectedVersion: 0,
	}); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("zero-version entitlement overwrite error = %v, want conflict", err)
	}
	entitlementMap, err = service.SaveUpstreamEntitlementMap(ctx, UpstreamEntitlementMapInput{
		EntitlementNo: "ent_support_01", UpstreamReference: "opaque_8087", Status: "active", ExpectedVersion: entitlementMap.Version,
	})
	if err != nil || entitlementMap.Version != 2 || entitlementMap.UpstreamReference != "opaque_8087" {
		t.Fatalf("update entitlement map = %+v, %v", entitlementMap, err)
	}
}
