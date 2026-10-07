package notification

import (
	"context"
	"encoding/json"
	"regexp"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	modeldo "lina-plugin-idc-notify/backend/internal/model/do"
)

var (
	templateVariablePattern = regexp.MustCompile(`\{\{([a-z][a-z0-9_]*)\}\}`)
	providerTemplatePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
)

type eventRow struct {
	EventCode         string `orm:"event_code"`
	TopicCode         string `orm:"topic_code"`
	ObjectType        string `orm:"object_type"`
	RequiredVariables string `orm:"required_variables"`
	OptionalVariables string `orm:"optional_variables"`
	SecurityRequired  bool   `orm:"security_required"`
	Version           int64  `orm:"version"`
}

type templateRow struct {
	TemplateNo                string    `orm:"template_no"`
	Version                   int64     `orm:"version"`
	EventCode                 string    `orm:"event_code"`
	ChannelCode               string    `orm:"channel_code"`
	LocaleCode                string    `orm:"locale_code"`
	SubjectTemplate           string    `orm:"subject_template"`
	BodyTemplate              string    `orm:"body_template"`
	ProviderCode              string    `orm:"provider_code"`
	ProviderTemplateID        string    `orm:"provider_template_id"`
	ProviderSMSVariableOrder  string    `orm:"provider_sms_variable_order"`
	ProviderSMSDataMapping    string    `orm:"provider_sms_data_mapping"`
	ProviderWeChatDataMapping string    `orm:"provider_wechat_data_mapping"`
	StatusCode                string    `orm:"status_code"`
	ContentDigest             string    `orm:"content_digest"`
	CreatedBy                 int64     `orm:"created_by"`
	CreatedAt                 time.Time `orm:"created_at"`
}

func (s *serviceImpl) ListEvents(ctx context.Context) ([]Event, error) {
	if _, err := s.authorize(ctx, "notification-events"); err != nil {
		return nil, err
	}
	var rows []eventRow
	if err := s.database.Model("plugin_idc_notify_event_catalog").Ctx(ctx).
		Where(tenantIDColumn, notificationTenantID).Where("enabled", true).
		OrderAsc("event_code").Limit(MaximumPageSize).Scan(&rows); err != nil {
		return nil, publicError(err)
	}
	items := make([]Event, len(rows))
	for index, row := range rows {
		var required, optional []string
		if json.Unmarshal([]byte(row.RequiredVariables), &required) != nil || json.Unmarshal([]byte(row.OptionalVariables), &optional) != nil {
			return nil, bizerr.NewCode(CodeUnavailable)
		}
		items[index] = Event{Code: row.EventCode, Topic: row.TopicCode, ObjectType: row.ObjectType, RequiredVariables: required, OptionalVariables: optional, SecurityRequired: row.SecurityRequired, Version: row.Version}
	}
	return items, nil
}

func (s *serviceImpl) ListTemplates(ctx context.Context, eventCode, channelCode string, limit int) ([]Template, error) {
	if _, err := s.authorize(ctx, "notification-templates"); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > MaximumPageSize || eventCode != "" && !stableCodePattern.MatchString(eventCode) || channelCode != "" && !stableCodePattern.MatchString(channelCode) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	model := s.database.Model("plugin_idc_notify_template_version").Ctx(ctx).
		Where(tenantIDColumn, notificationTenantID)
	if eventCode != "" {
		model = model.Where("event_code", eventCode)
	}
	if channelCode != "" {
		model = model.Where("channel_code", channelCode)
	}
	var rows []templateRow
	if err := model.OrderDesc("created_at").OrderDesc("template_no").Limit(limit).Scan(&rows); err != nil {
		return nil, publicError(err)
	}
	return projectTemplates(rows), nil
}

func (s *serviceImpl) GetTemplate(ctx context.Context, templateNo string, version int64) (*Template, error) {
	if _, err := s.authorize(ctx, "notification-template-"+templateNo); err != nil {
		return nil, err
	}
	if !stableCodePattern.MatchString(templateNo) || version < 0 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	row, err := s.loadTemplate(ctx, templateNo, version)
	if err != nil || row == nil {
		return nil, errorOrCode(err, CodeNotAccessible)
	}
	item := projectTemplate(*row)
	return &item, nil
}

func (s *serviceImpl) SaveTemplate(ctx context.Context, input TemplateWriteInput) (*Template, error) {
	actor, err := s.authorize(ctx, "notification-templates")
	if err != nil {
		return nil, err
	}
	if !validTemplateWriteInput(input) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	event, err := s.loadEvent(ctx, input.EventCode)
	if err != nil || event == nil {
		return nil, errorOrCode(err, CodeInvalid)
	}
	if err = validateTemplateVariables(input.Subject+"\n"+input.Body, event); err != nil {
		return nil, err
	}
	providerCode := ""
	if input.ChannelCode == "sms" {
		channel, channelErr := s.loadDeliveryChannel(ctx, "sms")
		if channelErr != nil || channel == nil || channel.ProviderCode == nil || channel.CredentialSecretRef == nil {
			return nil, errorOrCode(channelErr, CodeChannelUnavailable)
		}
		providerCode = *channel.ProviderCode
	}
	providerTemplateID, smsVariableOrder, smsDataMapping, wechatDataMapping, err := validateProviderMapping(input, event, providerCode)
	if err != nil {
		return nil, err
	}
	templateNo, err := resolveTemplateNo(input)
	if err != nil {
		return nil, err
	}
	digest := digestText(input.EventCode, input.ChannelCode, input.Locale, input.Subject, input.Body, input.Status,
		providerCode, providerTemplateID, smsVariableOrder, smsDataMapping, wechatDataMapping)
	var createdVersion int64
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var latest templateRow
		if loadErr := s.database.Model("plugin_idc_notify_template_version").Ctx(ctx).
			Where(tenantIDColumn, notificationTenantID).Where("template_no", templateNo).
			OrderDesc("version").Limit(1).LockUpdate().Scan(&latest); loadErr != nil {
			return loadErr
		}
		if latest.TemplateNo == "" {
			if input.ExpectedVersion != 0 {
				return bizerr.NewCode(CodeConflict)
			}
			createdVersion = 1
		} else {
			if latest.ContentDigest == digest {
				createdVersion = latest.Version
				return nil
			}
			if latest.Version != input.ExpectedVersion || latest.EventCode != input.EventCode || latest.ChannelCode != input.ChannelCode || latest.LocaleCode != input.Locale {
				return bizerr.NewCode(CodeConflict)
			}
			createdVersion = latest.Version + 1
		}
		if input.Status == "active" {
			if _, updateErr := s.database.Model("plugin_idc_notify_template_version").Ctx(ctx).
				Where(tenantIDColumn, notificationTenantID).Where("event_code", input.EventCode).
				Where("channel_code", input.ChannelCode).
				Where("locale_code", input.Locale).Where("status_code", "active").
				Data(modeldo.TemplateVersion{StatusCode: "disabled"}).Update(); updateErr != nil {
				return updateErr
			}
		}
		if _, insertErr := s.database.Model("plugin_idc_notify_template_version").Ctx(ctx).Data(modeldo.TemplateVersion{
			TenantId: notificationTenantID, TemplateNo: templateNo,
			Version: createdVersion, EventCode: input.EventCode,
			ChannelCode: input.ChannelCode, LocaleCode: input.Locale,
			SubjectTemplate: input.Subject, BodyTemplate: input.Body,
			ProviderCode: providerCode, ProviderTemplateId: providerTemplateID,
			ProviderSmsVariableOrder: smsVariableOrder, ProviderSmsDataMapping: smsDataMapping,
			ProviderWechatDataMapping: wechatDataMapping,
			StatusCode:                input.Status, ContentDigest: digest, CreatedBy: actor,
		}).Insert(); insertErr != nil {
			return insertErr
		}
		return s.management.Audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "notification.template.save", IdempotencyKey: input.IdempotencyKey,
			OperationCode: "notification.template.save", TargetType: "notification_template", TargetNo: templateNo,
			ResultCode: "succeeded", ReasonCode: "merchant_template_save", RiskCode: "normal",
			RequiredPermission: PermissionChannelManage,
			BeforeSnapshot:     map[string]any{"version": input.ExpectedVersion}, AfterSnapshot: map[string]any{"version": createdVersion, "status": input.Status},
		})
	})
	if err != nil {
		return nil, publicError(err)
	}
	row, err := s.loadTemplate(ctx, templateNo, createdVersion)
	if err != nil || row == nil {
		return nil, publicError(errorOrCode(err, CodeUnavailable))
	}
	item := projectTemplate(*row)
	return &item, nil
}

func validTemplateWriteInput(input TemplateWriteInput) bool {
	contentLength := len(input.Subject) + len(input.Body)
	return validateIdempotencyKey(input.IdempotencyKey) && stableCodePattern.MatchString(input.EventCode) &&
		stableCodePattern.MatchString(input.ChannelCode) && validLocale(input.Locale) &&
		validTemplateStatus(input.Status) && contentLength > 0 && contentLength <= MaximumTemplateBytes &&
		input.ExpectedVersion >= 0
}

func resolveTemplateNo(input TemplateWriteInput) (string, error) {
	if input.TemplateNo == "" {
		if input.ExpectedVersion != 0 {
			return "", bizerr.NewCode(CodeConflict)
		}
		return stableNumber("ntpl_", input.IdempotencyKey, input.EventCode, input.ChannelCode, input.Locale), nil
	}
	if !stableCodePattern.MatchString(input.TemplateNo) || input.ExpectedVersion <= 0 {
		return "", bizerr.NewCode(CodeInvalid)
	}
	return input.TemplateNo, nil
}

func (s *serviceImpl) PreviewTemplate(ctx context.Context, input TemplatePreviewInput) (*TemplatePreview, error) {
	if _, err := s.authorize(ctx, "notification-template-"+input.TemplateNo); err != nil {
		return nil, err
	}
	if len(input.Variables) > MaximumVariables {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	row, err := s.loadTemplate(ctx, input.TemplateNo, input.Version)
	if err != nil || row == nil {
		return nil, errorOrCode(err, CodeNotAccessible)
	}
	event, err := s.loadEvent(ctx, row.EventCode)
	if err != nil || event == nil {
		return nil, errorOrCode(err, CodeUnavailable)
	}
	allowed := eventVariableSet(event)
	for key, value := range input.Variables {
		if _, ok := allowed[key]; !ok || len(value) > 4096 {
			return nil, bizerr.NewCode(CodeInvalid)
		}
	}
	subject, err := renderTemplate(row.SubjectTemplate, input.Variables)
	if err != nil {
		return nil, err
	}
	body, err := renderTemplate(row.BodyTemplate, input.Variables)
	if err != nil {
		return nil, err
	}
	return &TemplatePreview{Subject: subject, Body: body}, nil
}

func (s *serviceImpl) loadEvent(ctx context.Context, code string) (*eventRow, error) {
	var row eventRow
	if err := s.database.Model("plugin_idc_notify_event_catalog").Ctx(ctx).
		Where(tenantIDColumn, notificationTenantID).Where("event_code", code).
		Where("enabled", true).Scan(&row); err != nil {
		return nil, err
	}
	if row.EventCode == "" {
		return nil, nil
	}
	return &row, nil
}

func (s *serviceImpl) loadTemplate(ctx context.Context, templateNo string, version int64) (*templateRow, error) {
	model := s.database.Model("plugin_idc_notify_template_version").Ctx(ctx).
		Where(tenantIDColumn, notificationTenantID).Where("template_no", templateNo)
	if version > 0 {
		model = model.Where("version", version)
	} else {
		model = model.OrderDesc("version").Limit(1)
	}
	var row templateRow
	if err := model.Scan(&row); err != nil {
		return nil, err
	}
	if row.TemplateNo == "" {
		return nil, nil
	}
	return &row, nil
}

func validateTemplateVariables(content string, event *eventRow) error {
	allowed := eventVariableSet(event)
	for _, match := range templateVariablePattern.FindAllStringSubmatch(content, MaximumVariables+1) {
		if len(match) != 2 {
			return bizerr.NewCode(CodeInvalid)
		}
		if _, ok := allowed[match[1]]; !ok {
			return bizerr.NewCode(CodeInvalid)
		}
	}
	if len(templateVariablePattern.FindAllStringSubmatch(content, MaximumVariables+1)) > MaximumVariables {
		return bizerr.NewCode(CodeInvalid)
	}
	return nil
}

func eventVariableSet(event *eventRow) map[string]struct{} {
	allowed := make(map[string]struct{})
	if event == nil {
		return allowed
	}
	var required, optional []string
	_ = json.Unmarshal([]byte(event.RequiredVariables), &required)
	_ = json.Unmarshal([]byte(event.OptionalVariables), &optional)
	for _, key := range append(required, optional...) {
		if variablePattern.MatchString(key) {
			allowed[key] = struct{}{}
		}
	}
	return allowed
}

func renderTemplate(content string, values map[string]string) (string, error) {
	missing := false
	rendered := templateVariablePattern.ReplaceAllStringFunc(content, func(match string) string {
		parts := templateVariablePattern.FindStringSubmatch(match)
		value, exists := values[parts[1]]
		if !exists {
			missing = true
			return match
		}
		return value
	})
	if missing {
		return "", bizerr.NewCode(CodeInvalid)
	}
	return rendered, nil
}

func projectTemplates(rows []templateRow) []Template {
	items := make([]Template, len(rows))
	for index, row := range rows {
		items[index] = projectTemplate(row)
	}
	return items
}

func projectTemplate(row templateRow) Template {
	item := Template{TemplateNo: row.TemplateNo, Version: row.Version, EventCode: row.EventCode, ChannelCode: row.ChannelCode, Locale: row.LocaleCode, Subject: row.SubjectTemplate, Body: row.BodyTemplate, Status: row.StatusCode, CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt.UTC()}
	switch row.ChannelCode {
	case "sms":
		var order []string
		mapping := make(map[string]string)
		if json.Unmarshal([]byte(row.ProviderSMSVariableOrder), &order) == nil &&
			json.Unmarshal([]byte(row.ProviderSMSDataMapping), &mapping) == nil {
			item.SMS = &SMSTemplateMapping{ProviderCode: row.ProviderCode, TemplateID: row.ProviderTemplateID,
				VariableOrder: order, DataMapping: mapping}
		}
	case "wechat_official":
		mapping := make(map[string]string)
		if json.Unmarshal([]byte(row.ProviderWeChatDataMapping), &mapping) == nil {
			item.WeChatOfficial = &WeChatOfficialTemplateMapping{TemplateID: row.ProviderTemplateID, DataMapping: mapping}
		}
	}
	return item
}

func validateProviderMapping(input TemplateWriteInput, event *eventRow, activeSMSProvider string) (string, string, string, string, error) {
	allowed := eventVariableSet(event)
	switch input.ChannelCode {
	case "sms":
		return validateSMSProviderMapping(input, allowed, activeSMSProvider)
	case "wechat_official":
		return validateWeChatProviderMapping(input, allowed)
	default:
		if input.SMS != nil || input.WeChatOfficial != nil {
			return "", "", "", "", bizerr.NewCode(CodeInvalid)
		}
		return "", "[]", "{}", "{}", nil
	}
}

func validateSMSProviderMapping(input TemplateWriteInput, allowed map[string]struct{}, activeSMSProvider string) (string, string, string, string, error) {
	if input.SMS == nil || input.WeChatOfficial != nil || input.SMS.ProviderCode != activeSMSProvider ||
		len(input.SMS.VariableOrder) > MaximumVariables || len(input.SMS.DataMapping) > MaximumVariables {
		return "", "", "", "", bizerr.NewCode(CodeInvalid)
	}
	if !validProviderVariables(input.SMS.VariableOrder, input.SMS.DataMapping, allowed) {
		return "", "", "", "", bizerr.NewCode(CodeInvalid)
	}
	if err := validateSMSProviderTemplate(input.SMS, activeSMSProvider); err != nil {
		return "", "", "", "", err
	}
	variableOrder := input.SMS.VariableOrder
	if variableOrder == nil {
		variableOrder = []string{}
	}
	dataMapping := input.SMS.DataMapping
	if dataMapping == nil {
		dataMapping = map[string]string{}
	}
	encodedOrder, err := json.Marshal(variableOrder)
	if err != nil {
		return "", "", "", "", bizerr.WrapCode(err, CodeInvalid)
	}
	encodedMapping, err := json.Marshal(dataMapping)
	if err != nil {
		return "", "", "", "", bizerr.WrapCode(err, CodeInvalid)
	}
	return input.SMS.TemplateID, string(encodedOrder), string(encodedMapping), "{}", nil
}

func validProviderVariables(order []string, mapping map[string]string, allowed map[string]struct{}) bool {
	for _, variable := range order {
		if _, ok := allowed[variable]; !ok {
			return false
		}
	}
	for key, variable := range mapping {
		if !providerDataKey(key) {
			return false
		}
		if _, ok := allowed[variable]; !ok {
			return false
		}
	}
	return true
}

func validateSMSProviderTemplate(mapping *SMSTemplateMapping, activeSMSProvider string) error {
	switch activeSMSProvider {
	case providerTencentSMS:
		if !numericTemplateID(mapping.TemplateID) || len(mapping.DataMapping) != 0 {
			return bizerr.NewCode(CodeInvalid)
		}
	case providerAliyunSMS:
		if !providerTemplatePattern.MatchString(mapping.TemplateID) || len(mapping.VariableOrder) != 0 {
			return bizerr.NewCode(CodeInvalid)
		}
	case providerSMSBao:
		if mapping.TemplateID != "" || len(mapping.VariableOrder) != 0 || len(mapping.DataMapping) != 0 {
			return bizerr.NewCode(CodeInvalid)
		}
	default:
		return bizerr.NewCode(CodeChannelUnavailable)
	}
	return nil
}

func validateWeChatProviderMapping(input TemplateWriteInput, allowed map[string]struct{}) (string, string, string, string, error) {
	if input.WeChatOfficial == nil || input.SMS != nil || input.WeChatOfficial.TemplateID == "" ||
		len(input.WeChatOfficial.TemplateID) > 128 || len(input.WeChatOfficial.DataMapping) == 0 ||
		len(input.WeChatOfficial.DataMapping) > MaximumVariables ||
		!validProviderVariables(nil, input.WeChatOfficial.DataMapping, allowed) {
		return "", "", "", "", bizerr.NewCode(CodeInvalid)
	}
	encoded, err := json.Marshal(input.WeChatOfficial.DataMapping)
	if err != nil {
		return "", "", "", "", bizerr.WrapCode(err, CodeInvalid)
	}
	return input.WeChatOfficial.TemplateID, "[]", "{}", string(encoded), nil
}

func numericTemplateID(value string) bool {
	if value == "" || len(value) > 32 {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func providerDataKey(value string) bool {
	if value == "" || len(value) > 64 || value[0] < 'A' && (value[0] < 'a' || value[0] > 'z') || value[0] > 'Z' && value[0] < 'a' {
		return false
	}
	for index := 1; index < len(value); index++ {
		character := value[index]
		if (character < 'A' || character > 'Z') && (character < 'a' || character > 'z') &&
			(character < '0' || character > '9') && character != '_' {
			return false
		}
	}
	return true
}

func validLocale(value string) bool { return value == "zh-CN" || value == "en-US" }
func validTemplateStatus(value string) bool {
	return value == "draft" || value == "active" || value == "disabled"
}
