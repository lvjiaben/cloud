package supportadmin

import (
	"context"

	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/supportdesk"
)

func (c *ControllerV1) SupportDepartmentSettingList(ctx context.Context, _ *v1.SupportDepartmentSettingListReq) (*v1.SupportDepartmentSettingListRes, error) {
	items, err := c.service.DepartmentSettings(ctx)
	if err != nil {
		return nil, err
	}
	return &v1.SupportDepartmentSettingListRes{Items: toDepartmentSettings(items)}, nil
}

func (c *ControllerV1) SupportDepartmentSettingCreate(ctx context.Context, req *v1.SupportDepartmentSettingCreateReq) (*v1.SupportDepartmentSettingCreateRes, error) {
	item, err := c.service.CreateDepartmentSetting(ctx, departmentSettingInput(req.OrgDeptID, req.NameZhCN, req.NameEnUS, req.ParentDepartmentNo, req.AssignmentMode, req.Status, 0))
	if err != nil {
		return nil, err
	}
	return &v1.SupportDepartmentSettingCreateRes{Item: toDepartmentSetting(*item)}, nil
}

func (c *ControllerV1) SupportDepartmentSettingUpdate(ctx context.Context, req *v1.SupportDepartmentSettingUpdateReq) (*v1.SupportDepartmentSettingUpdateRes, error) {
	item, err := c.service.UpdateDepartmentSetting(ctx, req.DepartmentNo, departmentSettingInput(req.OrgDeptID, req.NameZhCN, req.NameEnUS, req.ParentDepartmentNo, req.AssignmentMode, req.Status, req.ExpectedVersion))
	if err != nil {
		return nil, err
	}
	return &v1.SupportDepartmentSettingUpdateRes{Item: toDepartmentSetting(*item)}, nil
}

func (c *ControllerV1) SupportDepartmentSettingDelete(ctx context.Context, req *v1.SupportDepartmentSettingDeleteReq) (*v1.SupportDepartmentSettingDeleteRes, error) {
	item, err := c.service.DisableDepartmentSetting(ctx, req.DepartmentNo, req.ExpectedVersion)
	if err != nil {
		return nil, err
	}
	return &v1.SupportDepartmentSettingDeleteRes{Item: toDepartmentSetting(*item)}, nil
}

func (c *ControllerV1) SupportIssueCategorySettingList(ctx context.Context, _ *v1.SupportIssueCategorySettingListReq) (*v1.SupportIssueCategorySettingListRes, error) {
	items, err := c.service.IssueCategorySettings(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]v1.SupportIssueCategorySetting, 0, len(items))
	for _, item := range items {
		result = append(result, *toIssueCategorySetting(item))
	}
	return &v1.SupportIssueCategorySettingListRes{Items: result}, nil
}

func (c *ControllerV1) SupportIssueCategorySettingCreate(ctx context.Context, req *v1.SupportIssueCategorySettingCreateReq) (*v1.SupportIssueCategorySettingCreateRes, error) {
	item, err := c.service.CreateIssueCategorySetting(ctx, issueCategorySettingInput(req.NameZhCN, req.NameEnUS, req.DepartmentNo, req.Priority, req.SortOrder, req.Status, 0))
	if err != nil {
		return nil, err
	}
	return &v1.SupportIssueCategorySettingCreateRes{Item: toIssueCategorySetting(*item)}, nil
}

func (c *ControllerV1) SupportIssueCategorySettingUpdate(ctx context.Context, req *v1.SupportIssueCategorySettingUpdateReq) (*v1.SupportIssueCategorySettingUpdateRes, error) {
	item, err := c.service.UpdateIssueCategorySetting(ctx, req.CategoryNo, issueCategorySettingInput(req.NameZhCN, req.NameEnUS, req.DepartmentNo, req.Priority, req.SortOrder, req.Status, req.ExpectedVersion))
	if err != nil {
		return nil, err
	}
	return &v1.SupportIssueCategorySettingUpdateRes{Item: toIssueCategorySetting(*item)}, nil
}

func (c *ControllerV1) SupportIssueCategorySettingDelete(ctx context.Context, req *v1.SupportIssueCategorySettingDeleteReq) (*v1.SupportIssueCategorySettingDeleteRes, error) {
	item, err := c.service.DisableIssueCategorySetting(ctx, req.CategoryNo, req.ExpectedVersion)
	if err != nil {
		return nil, err
	}
	return &v1.SupportIssueCategorySettingDeleteRes{Item: toIssueCategorySetting(*item)}, nil
}

func (c *ControllerV1) SupportCustomStatusSettingList(ctx context.Context, _ *v1.SupportCustomStatusSettingListReq) (*v1.SupportCustomStatusSettingListRes, error) {
	items, err := c.service.CustomStatusSettings(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]v1.SupportCustomStatusSetting, 0, len(items))
	for _, item := range items {
		result = append(result, *toCustomStatusSetting(item))
	}
	return &v1.SupportCustomStatusSettingListRes{Items: result}, nil
}

func (c *ControllerV1) SupportCustomStatusSettingCreate(ctx context.Context, req *v1.SupportCustomStatusSettingCreateReq) (*v1.SupportCustomStatusSettingCreateRes, error) {
	item, err := c.service.CreateCustomStatusSetting(ctx, customStatusSettingInput(req.NameZhCN, req.NameEnUS, req.BaseStage, req.SortOrder, req.Status, 0))
	if err != nil {
		return nil, err
	}
	return &v1.SupportCustomStatusSettingCreateRes{Item: toCustomStatusSetting(*item)}, nil
}

func (c *ControllerV1) SupportCustomStatusSettingUpdate(ctx context.Context, req *v1.SupportCustomStatusSettingUpdateReq) (*v1.SupportCustomStatusSettingUpdateRes, error) {
	item, err := c.service.UpdateCustomStatusSetting(ctx, req.StatusNo, customStatusSettingInput(req.NameZhCN, req.NameEnUS, req.BaseStage, req.SortOrder, req.Status, req.ExpectedVersion))
	if err != nil {
		return nil, err
	}
	return &v1.SupportCustomStatusSettingUpdateRes{Item: toCustomStatusSetting(*item)}, nil
}

func (c *ControllerV1) SupportCustomStatusSettingDelete(ctx context.Context, req *v1.SupportCustomStatusSettingDeleteReq) (*v1.SupportCustomStatusSettingDeleteRes, error) {
	item, err := c.service.DisableCustomStatusSetting(ctx, req.StatusNo, req.ExpectedVersion)
	if err != nil {
		return nil, err
	}
	return &v1.SupportCustomStatusSettingDeleteRes{Item: toCustomStatusSetting(*item)}, nil
}

func (c *ControllerV1) SupportReplyPresetSettingList(ctx context.Context, _ *v1.SupportReplyPresetSettingListReq) (*v1.SupportReplyPresetSettingListRes, error) {
	items, err := c.service.ReplyPresetSettings(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]v1.SupportReplyPresetSetting, 0, len(items))
	for _, item := range items {
		result = append(result, *toReplyPresetSetting(item))
	}
	return &v1.SupportReplyPresetSettingListRes{Items: result}, nil
}

func (c *ControllerV1) SupportReplyPresetSettingCreate(ctx context.Context, req *v1.SupportReplyPresetSettingCreateReq) (*v1.SupportReplyPresetSettingCreateRes, error) {
	item, err := c.service.CreateReplyPresetSetting(ctx, replyPresetSettingInput(req.DepartmentNo, req.Locale, req.Title, req.Body, req.SortOrder, req.Status, 0))
	if err != nil {
		return nil, err
	}
	return &v1.SupportReplyPresetSettingCreateRes{Item: toReplyPresetSetting(*item)}, nil
}

func (c *ControllerV1) SupportReplyPresetSettingUpdate(ctx context.Context, req *v1.SupportReplyPresetSettingUpdateReq) (*v1.SupportReplyPresetSettingUpdateRes, error) {
	item, err := c.service.UpdateReplyPresetSetting(ctx, req.PresetNo, replyPresetSettingInput(req.DepartmentNo, req.Locale, req.Title, req.Body, req.SortOrder, req.Status, req.ExpectedVersion))
	if err != nil {
		return nil, err
	}
	return &v1.SupportReplyPresetSettingUpdateRes{Item: toReplyPresetSetting(*item)}, nil
}

func (c *ControllerV1) SupportReplyPresetSettingDelete(ctx context.Context, req *v1.SupportReplyPresetSettingDeleteReq) (*v1.SupportReplyPresetSettingDeleteRes, error) {
	item, err := c.service.DisableReplyPresetSetting(ctx, req.PresetNo, req.ExpectedVersion)
	if err != nil {
		return nil, err
	}
	return &v1.SupportReplyPresetSettingDeleteRes{Item: toReplyPresetSetting(*item)}, nil
}

func (c *ControllerV1) SupportSLACalendarSettingList(ctx context.Context, _ *v1.SupportSLACalendarSettingListReq) (*v1.SupportSLACalendarSettingListRes, error) {
	items, err := c.service.SLACalendarSettings(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]v1.SupportSLACalendarSetting, 0, len(items))
	for _, item := range items {
		result = append(result, *toSLACalendarSetting(item))
	}
	return &v1.SupportSLACalendarSettingListRes{Items: result}, nil
}

func (c *ControllerV1) SupportSLACalendarSettingCreate(ctx context.Context, req *v1.SupportSLACalendarSettingCreateReq) (*v1.SupportSLACalendarSettingCreateRes, error) {
	item, err := c.service.CreateSLACalendarSetting(ctx, slaCalendarSettingInput(req.Name, req.Mode, req.Timezone, req.Workdays, req.WorkStartMinute, req.WorkEndMinute, req.Status, 0))
	if err != nil {
		return nil, err
	}
	return &v1.SupportSLACalendarSettingCreateRes{Item: toSLACalendarSetting(*item)}, nil
}

func (c *ControllerV1) SupportSLACalendarSettingUpdate(ctx context.Context, req *v1.SupportSLACalendarSettingUpdateReq) (*v1.SupportSLACalendarSettingUpdateRes, error) {
	item, err := c.service.UpdateSLACalendarSetting(ctx, req.CalendarNo, slaCalendarSettingInput(req.Name, req.Mode, req.Timezone, req.Workdays, req.WorkStartMinute, req.WorkEndMinute, req.Status, req.ExpectedVersion))
	if err != nil {
		return nil, err
	}
	return &v1.SupportSLACalendarSettingUpdateRes{Item: toSLACalendarSetting(*item)}, nil
}

func (c *ControllerV1) SupportSLACalendarSettingDelete(ctx context.Context, req *v1.SupportSLACalendarSettingDeleteReq) (*v1.SupportSLACalendarSettingDeleteRes, error) {
	item, err := c.service.DisableSLACalendarSetting(ctx, req.CalendarNo, req.ExpectedVersion)
	if err != nil {
		return nil, err
	}
	return &v1.SupportSLACalendarSettingDeleteRes{Item: toSLACalendarSetting(*item)}, nil
}

func (c *ControllerV1) SupportSLAHolidaySettingList(ctx context.Context, req *v1.SupportSLAHolidaySettingListReq) (*v1.SupportSLAHolidaySettingListRes, error) {
	items, err := c.service.SLAHolidaySettings(ctx, req.CalendarNo)
	if err != nil {
		return nil, err
	}
	result := make([]v1.SupportSLAHolidaySetting, 0, len(items))
	for _, item := range items {
		result = append(result, v1.SupportSLAHolidaySetting{CalendarNo: item.CalendarNo, Date: item.Date, Name: item.Name})
	}
	return &v1.SupportSLAHolidaySettingListRes{Items: result}, nil
}

func (c *ControllerV1) SupportSLAHolidaySettingPut(ctx context.Context, req *v1.SupportSLAHolidaySettingPutReq) (*v1.SupportSLAHolidaySettingPutRes, error) {
	item, err := c.service.PutSLAHolidaySetting(ctx, req.CalendarNo, supportdesk.SLAHolidaySettingInput{Date: req.Date, Name: req.Name})
	if err != nil {
		return nil, err
	}
	return &v1.SupportSLAHolidaySettingPutRes{Item: &v1.SupportSLAHolidaySetting{CalendarNo: item.CalendarNo, Date: item.Date, Name: item.Name}}, nil
}

func (c *ControllerV1) SupportSLAHolidaySettingDelete(ctx context.Context, req *v1.SupportSLAHolidaySettingDeleteReq) (*v1.SupportSLAHolidaySettingDeleteRes, error) {
	if err := c.service.DeleteSLAHolidaySetting(ctx, req.CalendarNo, req.Date); err != nil {
		return nil, err
	}
	return &v1.SupportSLAHolidaySettingDeleteRes{}, nil
}

func (c *ControllerV1) SupportSLARuleSettingList(ctx context.Context, _ *v1.SupportSLARuleSettingListReq) (*v1.SupportSLARuleSettingListRes, error) {
	items, err := c.service.SLARuleSettings(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]v1.SupportSLARuleSetting, 0, len(items))
	for _, item := range items {
		result = append(result, *toSLARuleSetting(item))
	}
	return &v1.SupportSLARuleSettingListRes{Items: result}, nil
}

func (c *ControllerV1) SupportSLARuleSettingCreate(ctx context.Context, req *v1.SupportSLARuleSettingCreateReq) (*v1.SupportSLARuleSettingCreateRes, error) {
	item, err := c.service.CreateSLARuleSetting(ctx, slaRuleSettingInput(req.DepartmentNo, req.CategoryNo, req.Priority, req.CalendarNo, req.FirstResponseMinutes, req.ResolutionMinutes, req.NearDueMinutes, req.Status, 0))
	if err != nil {
		return nil, err
	}
	return &v1.SupportSLARuleSettingCreateRes{Item: toSLARuleSetting(*item)}, nil
}

func (c *ControllerV1) SupportSLARuleSettingUpdate(ctx context.Context, req *v1.SupportSLARuleSettingUpdateReq) (*v1.SupportSLARuleSettingUpdateRes, error) {
	item, err := c.service.UpdateSLARuleSetting(ctx, req.RuleNo, slaRuleSettingInput(req.DepartmentNo, req.CategoryNo, req.Priority, req.CalendarNo, req.FirstResponseMinutes, req.ResolutionMinutes, req.NearDueMinutes, req.Status, req.ExpectedVersion))
	if err != nil {
		return nil, err
	}
	return &v1.SupportSLARuleSettingUpdateRes{Item: toSLARuleSetting(*item)}, nil
}

func (c *ControllerV1) SupportSLARuleSettingDelete(ctx context.Context, req *v1.SupportSLARuleSettingDeleteReq) (*v1.SupportSLARuleSettingDeleteRes, error) {
	item, err := c.service.DisableSLARuleSetting(ctx, req.RuleNo, req.ExpectedVersion)
	if err != nil {
		return nil, err
	}
	return &v1.SupportSLARuleSettingDeleteRes{Item: toSLARuleSetting(*item)}, nil
}

func departmentSettingInput(orgDeptID int, nameZhCN, nameEnUS, parentDepartmentNo, assignmentMode, status string, expectedVersion int64) supportdesk.DepartmentSettingInput {
	return supportdesk.DepartmentSettingInput{OrgDeptID: orgDeptID, NameZhCN: nameZhCN, NameEnUS: nameEnUS, ParentDepartmentNo: parentDepartmentNo, AssignmentMode: supportdesk.AssignmentMode(assignmentMode), Status: supportdesk.SettingStatus(status), ExpectedVersion: expectedVersion}
}

func issueCategorySettingInput(nameZhCN, nameEnUS, departmentNo, priority string, sortOrder int, status string, expectedVersion int64) supportdesk.IssueCategorySettingInput {
	return supportdesk.IssueCategorySettingInput{NameZhCN: nameZhCN, NameEnUS: nameEnUS, DepartmentNo: departmentNo, Priority: priority, SortOrder: sortOrder, Status: supportdesk.SettingStatus(status), ExpectedVersion: expectedVersion}
}

func customStatusSettingInput(nameZhCN, nameEnUS, baseStage string, sortOrder int, status string, expectedVersion int64) supportdesk.CustomStatusSettingInput {
	return supportdesk.CustomStatusSettingInput{NameZhCN: nameZhCN, NameEnUS: nameEnUS, BaseStage: baseStage, SortOrder: sortOrder, Status: supportdesk.SettingStatus(status), ExpectedVersion: expectedVersion}
}

func replyPresetSettingInput(departmentNo, locale, title, body string, sortOrder int, status string, expectedVersion int64) supportdesk.ReplyPresetSettingInput {
	return supportdesk.ReplyPresetSettingInput{DepartmentNo: departmentNo, Locale: locale, Title: title, Body: body, SortOrder: sortOrder, Status: supportdesk.SettingStatus(status), ExpectedVersion: expectedVersion}
}

func slaCalendarSettingInput(name, mode, timezone string, workdays []int, start, end *int, status string, expectedVersion int64) supportdesk.SLACalendarSettingInput {
	return supportdesk.SLACalendarSettingInput{Name: name, Mode: mode, Timezone: timezone, Workdays: workdays, WorkStartMinute: start, WorkEndMinute: end, Status: supportdesk.SettingStatus(status), ExpectedVersion: expectedVersion}
}

func slaRuleSettingInput(departmentNo, categoryNo, priority, calendarNo string, firstResponse, resolution *int, nearDue []int, status string, expectedVersion int64) supportdesk.SLARuleSettingInput {
	return supportdesk.SLARuleSettingInput{DepartmentNo: departmentNo, CategoryNo: categoryNo, Priority: priority, CalendarNo: calendarNo, FirstResponseMinutes: firstResponse, ResolutionMinutes: resolution, NearDueMinutes: nearDue, Status: supportdesk.SettingStatus(status), ExpectedVersion: expectedVersion}
}

func toDepartmentSettings(items []supportdesk.DepartmentOption) []v1.SupportDepartmentSetting {
	result := make([]v1.SupportDepartmentSetting, 0, len(items))
	for _, item := range items {
		result = append(result, *toDepartmentSetting(item))
	}
	return result
}

func toDepartmentSetting(item supportdesk.DepartmentOption) *v1.SupportDepartmentSetting {
	return &v1.SupportDepartmentSetting{OrgDeptID: item.OrgDeptID, OrgDeptName: item.OrgDeptName, DepartmentNo: item.DepartmentNo, NameZhCN: item.NameZhCN, NameEnUS: item.NameEnUS, ParentDepartmentNo: item.ParentDepartmentNo, AssignmentMode: item.AssignmentMode, Status: item.Status, Version: item.Version}
}

func toIssueCategorySetting(item supportdesk.IssueCategorySetting) *v1.SupportIssueCategorySetting {
	return &v1.SupportIssueCategorySetting{CategoryNo: item.CategoryNo, NameZhCN: item.NameZhCN, NameEnUS: item.NameEnUS, DepartmentNo: item.DepartmentNo, Priority: item.Priority, SortOrder: item.SortOrder, Status: string(item.Status), Version: item.Version}
}

func toCustomStatusSetting(item supportdesk.CustomStatusSetting) *v1.SupportCustomStatusSetting {
	return &v1.SupportCustomStatusSetting{StatusNo: item.StatusNo, NameZhCN: item.NameZhCN, NameEnUS: item.NameEnUS, BaseStage: item.BaseStage, SortOrder: item.SortOrder, Status: string(item.Status), Version: item.Version}
}

func toReplyPresetSetting(item supportdesk.ReplyPresetSetting) *v1.SupportReplyPresetSetting {
	return &v1.SupportReplyPresetSetting{PresetNo: item.PresetNo, DepartmentNo: item.DepartmentNo, Locale: item.Locale, Title: item.Title, Body: item.Body, SortOrder: item.SortOrder, Status: string(item.Status), Version: item.Version}
}

func toSLACalendarSetting(item supportdesk.SLACalendarSetting) *v1.SupportSLACalendarSetting {
	return &v1.SupportSLACalendarSetting{CalendarNo: item.CalendarNo, Name: item.Name, Mode: item.Mode, Timezone: item.Timezone, Workdays: append([]int(nil), item.Workdays...), WorkStartMinute: item.WorkStartMinute, WorkEndMinute: item.WorkEndMinute, Status: string(item.Status), Version: item.Version}
}

func toSLARuleSetting(item supportdesk.SLARuleSetting) *v1.SupportSLARuleSetting {
	return &v1.SupportSLARuleSetting{RuleNo: item.RuleNo, DepartmentNo: item.DepartmentNo, CategoryNo: item.CategoryNo, Priority: item.Priority, CalendarNo: item.CalendarNo, FirstResponseMinutes: item.FirstResponseMinutes, ResolutionMinutes: item.ResolutionMinutes, NearDueMinutes: append([]int(nil), item.NearDueMinutes...), Status: string(item.Status), Version: item.Version}
}
