// This file owns support custom-field configuration, typed creation validation, and immutable snapshot persistence.
package supportdesk

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
)

const (
	customFieldSingleLine   = "single_line"
	customFieldMultiLine    = "multi_line"
	customFieldNumber       = "number"
	customFieldSingleSelect = "single_select"
	customFieldMultiSelect  = "multi_select"
	customFieldDate         = "date"
	customFieldBoolean      = "boolean"
	customFieldCustomer     = "customer_visible"
	customFieldInternal     = "internal_only"
)

var customFieldNumberPattern = regexp.MustCompile(`^-?[0-9]{1,20}(\.[0-9]{1,10})?$`)

// PublicPresalesCustomFields returns active public definitions without internal configuration fields.
func (s *serviceImpl) PublicPresalesCustomFields(ctx context.Context) ([]CustomFieldDefinition, error) {
	return s.loadCustomFieldDefinitions(ctx, "presales", customFieldCustomer, false, false)
}

// AccountCustomFields authenticates the current customer before returning active customer-ticket definitions.
func (s *serviceImpl) AccountCustomFields(ctx context.Context) ([]CustomFieldDefinition, error) {
	if _, err := s.customerActor(ctx); err != nil {
		return nil, err
	}
	return s.loadCustomFieldDefinitions(ctx, "customer", customFieldCustomer, false, false)
}

// StaffInternalCustomFields requires internal creation permission before returning internal-only definitions.
func (s *serviceImpl) StaffInternalCustomFields(ctx context.Context) ([]CustomFieldDefinition, error) {
	if _, err := s.staffActor(ctx, PermissionCreate); err != nil {
		return nil, err
	}
	return s.loadCustomFieldDefinitions(ctx, "internal", customFieldInternal, false, false)
}

// CustomFieldSettings requires configuration permission and includes disabled definitions for administration.
func (s *serviceImpl) CustomFieldSettings(ctx context.Context) ([]CustomFieldDefinition, error) {
	if _, err := s.staffActor(ctx, PermissionConfigure); err != nil {
		return nil, err
	}
	return s.loadCustomFieldDefinitions(ctx, "", "", true, false)
}

// CreateCustomFieldSetting validates and atomically inserts one bilingual definition and its options.
func (s *serviceImpl) CreateCustomFieldSetting(ctx context.Context, input CustomFieldSettingInput) (*CustomFieldDefinition, error) {
	a, err := s.staffActor(ctx, PermissionConfigure)
	if err != nil {
		return nil, err
	}
	input, ok := normalizeCustomFieldSetting(input, false)
	if !ok {
		return nil, bizInvalid()
	}
	fieldNo, err := s.newNo("SCF_")
	if err != nil {
		return nil, wrapUnavailable(err, "generate support custom field")
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if _, lockErr := s.database.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('idc-core:support-custom-field-settings',0))`); lockErr != nil {
			return lockErr
		}
		count, countErr := queryCount(ctx, s.database, `SELECT COUNT(1) FROM plugin_idc_core_support_custom_field_definition WHERE tenant_id=0`)
		if countErr != nil {
			return countErr
		}
		if count >= maxCustomFieldSettings {
			return bizerr.NewCode(CodeConflict)
		}
		if _, insertErr := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_support_custom_field_definition (tenant_id,field_no,field_type,name_zh_cn,name_en_us,description_zh_cn,description_en_us,applies_presales,applies_customer,applies_internal,visibility_code,required,sort_order,status_code,version,created_by,updated_by) VALUES (0,?,?,?,?,?,?,?,?,?,?,?,?,?,1,?,?)`, fieldNo, input.FieldType, input.NameZhCN, input.NameEnUS, input.DescriptionZhCN, input.DescriptionEnUS, input.AppliesPresales, input.AppliesCustomer, input.AppliesInternal, input.Visibility, input.Required, input.SortOrder, input.Status, a.UserID, a.UserID); insertErr != nil {
			return insertErr
		}
		return s.insertCustomFieldOptions(ctx, fieldNo, input.Options, a.UserID)
	})
	if err != nil {
		return nil, wrapUnavailable(err, "create support custom field")
	}
	return s.customFieldSetting(ctx, fieldNo)
}

// UpdateCustomFieldSetting uses optimistic concurrency and refuses type changes after the definition is referenced.
func (s *serviceImpl) UpdateCustomFieldSetting(ctx context.Context, fieldNo string, input CustomFieldSettingInput) (*CustomFieldDefinition, error) {
	a, err := s.staffActor(ctx, PermissionConfigure)
	if err != nil {
		return nil, err
	}
	input, ok := normalizeCustomFieldSetting(input, true)
	if !ok || !businessNoPattern.MatchString(fieldNo) {
		return nil, bizInvalid()
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var rows []struct {
			FieldType string `orm:"field_type"`
			Version   int64  `orm:"version"`
		}
		if loadErr := s.database.GetScan(ctx, &rows, `SELECT field_type,version FROM plugin_idc_core_support_custom_field_definition WHERE tenant_id=0 AND field_no=? FOR UPDATE`, fieldNo); loadErr != nil {
			return loadErr
		}
		if len(rows) != 1 {
			return bizNotAccessible()
		}
		if rows[0].Version != input.ExpectedVersion {
			return bizerr.NewCode(CodeConflict)
		}
		referenced, refErr := queryCount(ctx, s.database, `SELECT COUNT(1) FROM plugin_idc_core_support_ticket_custom_field_value WHERE tenant_id=0 AND field_no=?`, fieldNo)
		if refErr != nil {
			return refErr
		}
		if referenced > 0 && rows[0].FieldType != input.FieldType {
			return bizerr.NewCode(CodeConflict)
		}
		result, updateErr := s.database.Exec(ctx, `UPDATE plugin_idc_core_support_custom_field_definition SET field_type=?,name_zh_cn=?,name_en_us=?,description_zh_cn=?,description_en_us=?,applies_presales=?,applies_customer=?,applies_internal=?,visibility_code=?,required=?,sort_order=?,status_code=?,version=version+1,updated_by=?,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND field_no=? AND version=?`, input.FieldType, input.NameZhCN, input.NameEnUS, input.DescriptionZhCN, input.DescriptionEnUS, input.AppliesPresales, input.AppliesCustomer, input.AppliesInternal, input.Visibility, input.Required, input.SortOrder, input.Status, a.UserID, fieldNo, input.ExpectedVersion)
		if updateErr != nil {
			return updateErr
		}
		if changed, rowsErr := result.RowsAffected(); rowsErr != nil || changed != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		return s.replaceCustomFieldOptions(ctx, fieldNo, input.Options, a.UserID)
	})
	if err != nil {
		return nil, wrapUnavailable(err, "update support custom field")
	}
	return s.customFieldSetting(ctx, fieldNo)
}

// DisableCustomFieldSetting uses optimistic concurrency to stop future submissions without deleting history.
func (s *serviceImpl) DisableCustomFieldSetting(ctx context.Context, fieldNo string, expectedVersion int64) (*CustomFieldDefinition, error) {
	a, err := s.staffActor(ctx, PermissionConfigure)
	if err != nil {
		return nil, err
	}
	if !businessNoPattern.MatchString(fieldNo) || expectedVersion <= 0 {
		return nil, bizInvalid()
	}
	result, err := s.database.Exec(ctx, `UPDATE plugin_idc_core_support_custom_field_definition SET status_code='disabled',version=version+1,updated_by=?,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND field_no=? AND version=?`, a.UserID, fieldNo, expectedVersion)
	if err != nil {
		return nil, wrapUnavailable(err, "disable support custom field")
	}
	if changed, rowsErr := result.RowsAffected(); rowsErr != nil || changed != 1 {
		return nil, bizerr.NewCode(CodeConflict)
	}
	return s.customFieldSetting(ctx, fieldNo)
}

func (s *serviceImpl) customFieldSetting(ctx context.Context, fieldNo string) (*CustomFieldDefinition, error) {
	items, err := s.loadCustomFieldDefinitions(ctx, "", "", true, false)
	if err != nil {
		return nil, err
	}
	for index := range items {
		if items[index].FieldNo == fieldNo {
			return &items[index], nil
		}
	}
	return nil, bizNotAccessible()
}

func (s *serviceImpl) loadCustomFieldDefinitions(ctx context.Context, ticketType, visibility string, includeDisabled, lock bool) ([]CustomFieldDefinition, error) {
	where := ` WHERE tenant_id=0`
	args := make([]any, 0, 2)
	if !includeDisabled {
		where += ` AND status_code='active'`
	}
	if visibility != "" {
		where += ` AND visibility_code=?`
		args = append(args, visibility)
	}
	switch ticketType {
	case "presales":
		where += ` AND applies_presales=TRUE`
	case "customer":
		where += ` AND applies_customer=TRUE`
	case "internal":
		where += ` AND applies_internal=TRUE`
	case "":
	default:
		return nil, bizInvalid()
	}
	args = append(args, maxCustomFieldSettings+1)
	var fields []CustomFieldDefinition
	lockClause := ""
	if lock {
		lockClause = " FOR SHARE"
	}
	if err := s.database.GetScan(ctx, &fields, `SELECT field_no,field_type,name_zh_cn,name_en_us,description_zh_cn,description_en_us,applies_presales,applies_customer,applies_internal,visibility_code AS visibility,required,sort_order,status_code AS status,version FROM plugin_idc_core_support_custom_field_definition`+where+` ORDER BY sort_order,id LIMIT ?`+lockClause, args...); err != nil {
		return nil, wrapUnavailable(err, "load support custom fields")
	}
	if len(fields) > maxCustomFieldSettings {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	if len(fields) == 0 {
		return []CustomFieldDefinition{}, nil
	}
	fieldNos := make([]string, 0, len(fields))
	byNo := make(map[string]*CustomFieldDefinition, len(fields))
	for index := range fields {
		fields[index].Options = []CustomFieldOption{}
		fieldNos = append(fieldNos, fields[index].FieldNo)
		byNo[fields[index].FieldNo] = &fields[index]
	}
	optionWhere := `tenant_id=0 AND field_no IN (` + sqlPlaceholders(len(fieldNos)) + `)`
	optionArgs := make([]any, 0, len(fieldNos))
	for _, fieldNo := range fieldNos {
		optionArgs = append(optionArgs, fieldNo)
	}
	if !includeDisabled {
		optionWhere += ` AND status_code='active'`
	}
	optionArgs = append(optionArgs, maxCustomFieldSettings*maxCustomOptionCount+1)
	var options []struct {
		FieldNo string `orm:"field_no"`
		CustomFieldOption
	}
	if err := s.database.GetScan(ctx, &options, `SELECT field_no,option_no,name_zh_cn,name_en_us,sort_order,status_code AS status,version FROM plugin_idc_core_support_custom_field_option WHERE `+optionWhere+` ORDER BY field_no,sort_order,id LIMIT ?`+lockClause, optionArgs...); err != nil {
		return nil, wrapUnavailable(err, "load support custom field options")
	}
	if len(options) > maxCustomFieldSettings*maxCustomOptionCount {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	optionCounts := make(map[string]int, len(fields))
	for _, option := range options {
		if field := byNo[option.FieldNo]; field != nil {
			optionCounts[option.FieldNo]++
			if optionCounts[option.FieldNo] > maxCustomOptionCount {
				return nil, bizerr.NewCode(CodeUnavailable)
			}
			field.Options = append(field.Options, option.CustomFieldOption)
		}
	}
	return fields, nil
}

func (s *serviceImpl) insertCustomFieldOptions(ctx context.Context, fieldNo string, options []CustomFieldOptionInput, actorUserID int) error {
	for _, option := range options {
		optionNo, err := s.newNo("SCO_")
		if err != nil {
			return err
		}
		if _, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_support_custom_field_option (tenant_id,option_no,field_no,name_zh_cn,name_en_us,sort_order,status_code,version,created_by,updated_by) VALUES (0,?,?,?,?,?,?,1,?,?)`, optionNo, fieldNo, option.NameZhCN, option.NameEnUS, option.SortOrder, option.Status, actorUserID, actorUserID); err != nil {
			return err
		}
	}
	return nil
}

func (s *serviceImpl) replaceCustomFieldOptions(ctx context.Context, fieldNo string, inputs []CustomFieldOptionInput, actorUserID int) error {
	var existing []struct {
		OptionNo string `orm:"option_no"`
		Version  int64  `orm:"version"`
	}
	if err := s.database.GetScan(ctx, &existing, `SELECT option_no,version FROM plugin_idc_core_support_custom_field_option WHERE tenant_id=0 AND field_no=? FOR UPDATE`, fieldNo); err != nil {
		return err
	}
	known := make(map[string]int64, len(existing))
	seen := make(map[string]struct{}, len(inputs))
	for _, item := range existing {
		known[item.OptionNo] = item.Version
	}
	for _, input := range inputs {
		if input.OptionNo == "" {
			if err := s.insertCustomFieldOptions(ctx, fieldNo, []CustomFieldOptionInput{input}, actorUserID); err != nil {
				return err
			}
			continue
		}
		version, ok := known[input.OptionNo]
		if !ok || input.ExpectedVersion != version {
			return bizerr.NewCode(CodeConflict)
		}
		if _, duplicate := seen[input.OptionNo]; duplicate {
			return bizInvalid()
		}
		seen[input.OptionNo] = struct{}{}
		result, err := s.database.Exec(ctx, `UPDATE plugin_idc_core_support_custom_field_option SET name_zh_cn=?,name_en_us=?,sort_order=?,status_code=?,version=version+1,updated_by=?,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND field_no=? AND option_no=? AND version=?`, input.NameZhCN, input.NameEnUS, input.SortOrder, input.Status, actorUserID, fieldNo, input.OptionNo, input.ExpectedVersion)
		if err != nil {
			return err
		}
		if changed, rowsErr := result.RowsAffected(); rowsErr != nil || changed != 1 {
			return bizerr.NewCode(CodeConflict)
		}
	}
	for optionNo, version := range known {
		if _, ok := seen[optionNo]; ok {
			continue
		}
		if _, err := s.database.Exec(ctx, `UPDATE plugin_idc_core_support_custom_field_option SET status_code='disabled',version=version+1,updated_by=?,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND field_no=? AND option_no=? AND version=?`, actorUserID, fieldNo, optionNo, version); err != nil {
			return err
		}
	}
	return nil
}

func normalizeCustomFieldSetting(input CustomFieldSettingInput, update bool) (CustomFieldSettingInput, bool) {
	input.NameZhCN, _ = normalizeText(input.NameZhCN, 128)
	input.NameEnUS, _ = normalizeText(input.NameEnUS, 128)
	input.DescriptionZhCN = strings.TrimSpace(input.DescriptionZhCN)
	input.DescriptionEnUS = strings.TrimSpace(input.DescriptionEnUS)
	if len([]byte(input.DescriptionZhCN)) > 512 || len([]byte(input.DescriptionEnUS)) > 512 {
		return input, false
	}
	if input.Status == "" {
		input.Status = SettingStatusActive
	}
	if !update {
		input.ExpectedVersion = 0
	}
	if !validCustomFieldType(input.FieldType) || input.NameZhCN == "" || input.NameEnUS == "" ||
		!validSettingStatus(input.Status) || input.SortOrder < 0 || (update && input.ExpectedVersion <= 0) {
		return input, false
	}
	if !validCustomFieldScope(input) {
		return input, false
	}
	selection := input.FieldType == customFieldSingleSelect || input.FieldType == customFieldMultiSelect
	if (!selection && len(input.Options) != 0) || len(input.Options) > maxCustomOptionCount {
		return input, false
	}
	input, activeOptions, ok := normalizeCustomFieldOptions(input)
	return input, ok && (!selection || input.Status == SettingStatusDisabled || activeOptions > 0)
}

func validCustomFieldScope(input CustomFieldSettingInput) bool {
	switch input.Visibility {
	case customFieldCustomer:
		return !input.AppliesInternal && (input.AppliesPresales || input.AppliesCustomer)
	case customFieldInternal:
		return input.AppliesInternal && !input.AppliesPresales && !input.AppliesCustomer
	default:
		return false
	}
}

func normalizeCustomFieldOptions(input CustomFieldSettingInput) (CustomFieldSettingInput, int, bool) {
	activeOptions := 0
	for index := range input.Options {
		option := &input.Options[index]
		option.NameZhCN, _ = normalizeText(option.NameZhCN, 128)
		option.NameEnUS, _ = normalizeText(option.NameEnUS, 128)
		if option.Status == "" {
			option.Status = SettingStatusActive
		}
		if option.NameZhCN == "" || option.NameEnUS == "" || option.SortOrder < 0 || !validSettingStatus(option.Status) ||
			(option.OptionNo != "" && (!businessNoPattern.MatchString(option.OptionNo) || option.ExpectedVersion <= 0)) ||
			(option.OptionNo == "" && option.ExpectedVersion != 0) {
			return input, 0, false
		}
		if option.Status == SettingStatusActive {
			activeOptions++
		}
	}
	return input, activeOptions, true
}

func validCustomFieldType(value string) bool {
	switch value {
	case customFieldSingleLine, customFieldMultiLine, customFieldNumber, customFieldSingleSelect,
		customFieldMultiSelect, customFieldDate, customFieldBoolean:
		return true
	default:
		return false
	}
}

func (s *serviceImpl) prepareCustomFieldSnapshots(ctx context.Context, ticketType string, submissions []CustomFieldSubmission) ([]CustomFieldValue, error) {
	if len(submissions) > maxCustomFieldCount {
		return nil, bizInvalid()
	}
	visibility := customFieldCustomer
	if ticketType == "internal" {
		visibility = customFieldInternal
	} else if ticketType != "customer" && ticketType != "presales" {
		return nil, bizInvalid()
	}
	definitions, err := s.loadCustomFieldDefinitions(ctx, ticketType, visibility, false, true)
	if err != nil {
		return nil, err
	}
	byNo := make(map[string]CustomFieldDefinition, len(definitions))
	for _, definition := range definitions {
		byNo[definition.FieldNo] = definition
	}
	submitted := make(map[string]CustomFieldSubmission, len(submissions))
	for _, submission := range submissions {
		if !businessNoPattern.MatchString(submission.FieldNo) {
			return nil, bizInvalid()
		}
		if _, duplicate := submitted[submission.FieldNo]; duplicate {
			return nil, bizInvalid()
		}
		submitted[submission.FieldNo] = submission
	}
	values := make([]CustomFieldValue, 0, len(submissions))
	for _, definition := range definitions {
		submission, exists := submitted[definition.FieldNo]
		if !exists {
			if definition.Required {
				return nil, bizInvalid()
			}
			continue
		}
		delete(submitted, definition.FieldNo)
		value, valid := normalizeCustomFieldSubmission(definition, submission)
		if !valid {
			return nil, bizInvalid()
		}
		values = append(values, value)
	}
	if len(submitted) != 0 {
		return nil, bizInvalid()
	}
	return values, nil
}

func normalizeCustomFieldSubmission(definition CustomFieldDefinition, input CustomFieldSubmission) (CustomFieldValue, bool) {
	value := CustomFieldValue{
		FieldNo: input.FieldNo, FieldType: definition.FieldType,
		NameZhCN: definition.NameZhCN, NameEnUS: definition.NameEnUS,
		DescriptionZhCN: definition.DescriptionZhCN, DescriptionEnUS: definition.DescriptionEnUS,
		Visibility: definition.Visibility, Required: definition.Required, SortOrder: definition.SortOrder,
		DefinitionVersion: definition.Version, SelectedOptions: []CustomFieldOption{},
	}
	source := customFieldSubmissionSource(input)
	switch definition.FieldType {
	case customFieldSingleLine, customFieldMultiLine:
		if source != customFieldSourceText {
			return value, false
		}
		limit := 1024
		if definition.FieldType == customFieldMultiLine {
			limit = maxTicketBodyBytes
		}
		normalized, ok := normalizeText(*input.TextValue, limit)
		value.NormalizedValue = normalized
		return value, ok
	case customFieldNumber:
		if source != customFieldSourceNumber {
			return value, false
		}
		normalized := normalizeDecimal(*input.NumberValue)
		value.NormalizedValue = normalized
		return value, normalized != ""
	case customFieldDate:
		if source != customFieldSourceDate {
			return value, false
		}
		normalized := strings.TrimSpace(*input.DateValue)
		parsed, err := time.Parse("2006-01-02", normalized)
		value.NormalizedValue = normalized
		return value, err == nil && parsed.Format("2006-01-02") == normalized
	case customFieldBoolean:
		if source != customFieldSourceBoolean {
			return value, false
		}
		if *input.BooleanValue {
			value.NormalizedValue = "true"
		} else {
			value.NormalizedValue = "false"
		}
		return value, true
	case customFieldSingleSelect, customFieldMultiSelect:
		if source != customFieldSourceOptions ||
			(definition.FieldType == customFieldSingleSelect && len(input.OptionNos) != 1) ||
			len(input.OptionNos) > 50 {
			return value, false
		}
		return normalizeSelectedCustomFieldOptions(value, definition.Options, input.OptionNos)
	default:
		return value, false
	}
}

const (
	customFieldSourceInvalid = iota
	customFieldSourceText
	customFieldSourceNumber
	customFieldSourceDate
	customFieldSourceBoolean
	customFieldSourceOptions
)

func customFieldSubmissionSource(input CustomFieldSubmission) int {
	source, members := customFieldSourceInvalid, 0
	if input.TextValue != nil {
		source, members = customFieldSourceText, members+1
	}
	if input.NumberValue != nil {
		source, members = customFieldSourceNumber, members+1
	}
	if input.DateValue != nil {
		source, members = customFieldSourceDate, members+1
	}
	if input.BooleanValue != nil {
		source, members = customFieldSourceBoolean, members+1
	}
	if len(input.OptionNos) > 0 {
		source, members = customFieldSourceOptions, members+1
	}
	if members != 1 {
		return customFieldSourceInvalid
	}
	return source
}

func normalizeSelectedCustomFieldOptions(
	value CustomFieldValue,
	available []CustomFieldOption,
	selectedNos []string,
) (CustomFieldValue, bool) {
	options := make(map[string]CustomFieldOption, len(available))
	for _, option := range available {
		options[option.OptionNo] = option
	}
	seen := make(map[string]struct{}, len(selectedNos))
	selected := make([]CustomFieldOption, 0, len(selectedNos))
	for _, optionNo := range selectedNos {
		option, ok := options[optionNo]
		if !ok {
			return value, false
		}
		if _, duplicate := seen[optionNo]; duplicate {
			return value, false
		}
		seen[optionNo] = struct{}{}
		selected = append(selected, option)
	}
	sort.Slice(selected, func(i, j int) bool {
		if selected[i].SortOrder == selected[j].SortOrder {
			return selected[i].OptionNo < selected[j].OptionNo
		}
		return selected[i].SortOrder < selected[j].SortOrder
	})
	optionNos := make([]string, 0, len(selected))
	for _, option := range selected {
		optionNos = append(optionNos, option.OptionNo)
	}
	value.NormalizedValue = strings.Join(optionNos, ",")
	value.SelectedOptions = selected
	return value, true
}

func normalizeDecimal(value string) string {
	value = strings.TrimSpace(value)
	if !customFieldNumberPattern.MatchString(value) {
		return ""
	}
	negative := strings.HasPrefix(value, "-")
	value = strings.TrimPrefix(value, "-")
	parts := strings.SplitN(value, ".", 2)
	integer := strings.TrimLeft(parts[0], "0")
	if integer == "" {
		integer = "0"
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = strings.TrimRight(parts[1], "0")
	}
	result := integer
	if fraction != "" {
		result += "." + fraction
	}
	if negative && result != "0" {
		result = "-" + result
	}
	return result
}

func (s *serviceImpl) saveCustomFieldSnapshots(ctx context.Context, ticketNo string, values []CustomFieldValue) error {
	if len(values) == 0 {
		return nil
	}
	valueSQL := strings.Builder{}
	valueSQL.WriteString(`INSERT INTO plugin_idc_core_support_ticket_custom_field_value (tenant_id,ticket_no,field_no,definition_version,field_type,name_zh_cn,name_en_us,description_zh_cn,description_en_us,visibility_code,required,sort_order,normalized_value,selected_option_count) VALUES `)
	valueArgs := make([]any, 0, len(values)*13)
	optionRows := 0
	for index, value := range values {
		if index > 0 {
			valueSQL.WriteByte(',')
		}
		valueSQL.WriteString(`(0,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
		valueArgs = append(valueArgs, ticketNo, value.FieldNo, value.DefinitionVersion, value.FieldType, value.NameZhCN, value.NameEnUS, value.DescriptionZhCN, value.DescriptionEnUS, value.Visibility, value.Required, value.SortOrder, value.NormalizedValue, len(value.SelectedOptions))
		optionRows += len(value.SelectedOptions)
	}
	if _, err := s.database.Exec(ctx, valueSQL.String(), valueArgs...); err != nil {
		return err
	}
	if optionRows == 0 {
		return nil
	}
	optionSQL := strings.Builder{}
	optionSQL.WriteString(`INSERT INTO plugin_idc_core_support_ticket_custom_field_value_option (tenant_id,ticket_no,field_no,option_no,name_zh_cn,name_en_us,sort_order) VALUES `)
	optionArgs := make([]any, 0, optionRows*6)
	row := 0
	for _, value := range values {
		for _, option := range value.SelectedOptions {
			if row > 0 {
				optionSQL.WriteByte(',')
			}
			optionSQL.WriteString(`(0,?,?,?,?,?,?)`)
			optionArgs = append(optionArgs, ticketNo, value.FieldNo, option.OptionNo, option.NameZhCN, option.NameEnUS, option.SortOrder)
			row++
		}
	}
	_, err := s.database.Exec(ctx, optionSQL.String(), optionArgs...)
	return err
}

func (s *serviceImpl) loadCustomFieldValues(ctx context.Context, ticketNo string, includeInternal bool) ([]CustomFieldValue, error) {
	where := `tenant_id=0 AND ticket_no=?`
	if !includeInternal {
		where += ` AND visibility_code='customer_visible'`
	}
	var values []CustomFieldValue
	if err := s.database.GetScan(ctx, &values, `SELECT field_no,definition_version,field_type,name_zh_cn,name_en_us,description_zh_cn,description_en_us,visibility_code AS visibility,required,sort_order,normalized_value FROM plugin_idc_core_support_ticket_custom_field_value WHERE `+where+` ORDER BY sort_order,id`, ticketNo); err != nil {
		return nil, wrapUnavailable(err, "load ticket custom fields")
	}
	if len(values) == 0 {
		return []CustomFieldValue{}, nil
	}
	byField := make(map[string]*CustomFieldValue, len(values))
	for index := range values {
		values[index].SelectedOptions = []CustomFieldOption{}
		byField[values[index].FieldNo] = &values[index]
	}
	var options []struct {
		FieldNo string `orm:"field_no"`
		CustomFieldOption
	}
	if err := s.database.GetScan(ctx, &options, `SELECT field_no,option_no,name_zh_cn,name_en_us,sort_order FROM plugin_idc_core_support_ticket_custom_field_value_option WHERE tenant_id=0 AND ticket_no=? ORDER BY field_no,sort_order,id`, ticketNo); err != nil {
		return nil, wrapUnavailable(err, "load ticket custom field option snapshots")
	}
	for _, option := range options {
		if value := byField[option.FieldNo]; value != nil {
			value.SelectedOptions = append(value.SelectedOptions, option.CustomFieldOption)
		}
	}
	return values, nil
}
