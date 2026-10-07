package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/notificationcap"
	modeldo "lina-plugin-idc-notify/backend/internal/model/do"
)

const eventMaterializationTable = "plugin_idc_notify_event_materialization"

type materializationReadRow struct {
	Ordinal                   int    `orm:"ordinal"`
	SourcePlugin              string `orm:"source_plugin"`
	SourceEventNo             string `orm:"source_event_no"`
	EventCode                 string `orm:"event_code"`
	TopicCode                 string `orm:"topic_code"`
	ObjectType                string `orm:"object_type"`
	RequiredVariables         string `orm:"required_variables"`
	OptionalVariables         string `orm:"optional_variables"`
	SecurityRequired          bool   `orm:"security_required"`
	RecipientKind             string `orm:"recipient_kind"`
	ChannelCode               string `orm:"channel_code"`
	PolicyEnabled             bool   `orm:"policy_enabled"`
	PolicyVersion             int64  `orm:"policy_version"`
	ChannelStatus             string `orm:"channel_status"`
	ChannelProviderCode       string `orm:"channel_provider_code"`
	TemplateNo                string `orm:"template_no"`
	TemplateVersion           int64  `orm:"template_version"`
	LocaleCode                string `orm:"locale_code"`
	SubjectTemplate           string `orm:"subject_template"`
	BodyTemplate              string `orm:"body_template"`
	TemplateProviderCode      string `orm:"template_provider_code"`
	ProviderTemplateID        string `orm:"provider_template_id"`
	ProviderSMSVariableOrder  string `orm:"provider_sms_variable_order"`
	ProviderSMSDataMapping    string `orm:"provider_sms_data_mapping"`
	ProviderWeChatDataMapping string `orm:"provider_wechat_data_mapping"`
	ExistingMaterializationNo string `orm:"existing_materialization_no"`
}

type materializationCandidate struct {
	claimOrdinal        int
	claim               notificationcap.ClaimedEvent
	event               eventRow
	policy              eventPolicyRow
	channel             string
	channelStatus       string
	channelProviderCode string
	templates           map[string]templateRow
	completed           bool
	projection          notificationcap.ContactProjection
	marketing           *notificationcap.MarketingSendDecision
	outcome             materializationOutcome
}

type materializationOutcome struct {
	status string
	reason string
	input  *EnqueueInput
}

type contactResolution struct {
	projection notificationcap.ContactProjection
}

func (s *serviceImpl) materializeEventBatch(ctx context.Context, claimed []notificationcap.ClaimedEvent) ([]bool, error) {
	if len(claimed) == 0 || len(claimed) > maximumEventMaterializationBatch {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	rows, err := s.readMaterializationBatch(ctx, claimed)
	if err != nil {
		return nil, publicError(err)
	}
	candidates, completed, err := buildMaterializationCandidates(claimed, rows)
	if err != nil {
		return nil, err
	}
	contactIndexes, contactRequests := materializationContactRequests(candidates)
	resolutions := make([]contactResolution, len(contactRequests))
	if len(contactRequests) > 0 {
		err = s.sources.Contacts.ResolveBatch(ctx, contactRequests, func(index int, projection notificationcap.ContactProjection, _ []byte) error {
			if index < 0 || index >= len(resolutions) {
				return bizerr.NewCode(CodeUnavailable)
			}
			resolutions[index] = contactResolution{projection: projection}
			return nil
		})
		if err != nil {
			for _, indexes := range contactIndexes {
				for _, candidateIndex := range indexes {
					candidates[candidateIndex].outcome = materializationOutcome{status: "failed", reason: "recipient_unavailable"}
				}
			}
		} else {
			for contactIndex, indexes := range contactIndexes {
				for _, candidateIndex := range indexes {
					candidates[candidateIndex].projection = resolutions[contactIndex].projection
				}
			}
		}
	}
	marketingIndexes, marketingInputs, err := prepareMaterializationInputs(candidates)
	if err != nil {
		return nil, err
	}
	if len(marketingInputs) > 0 {
		decisions, gateErr := s.sources.Marketing.CheckSendBatch(ctx, marketingInputs)
		if gateErr != nil || len(decisions) != len(marketingInputs) {
			for _, candidateIndex := range marketingIndexes {
				candidates[candidateIndex].outcome = materializationOutcome{status: "failed", reason: "marketing_unavailable"}
			}
		} else {
			for index, candidateIndex := range marketingIndexes {
				decision := decisions[index]
				if !decision.Allowed || decision.DecisionDigest == "" {
					candidates[candidateIndex].outcome = materializationOutcome{status: "skipped", reason: "marketing_blocked"}
					continue
				}
				candidates[candidateIndex].marketing = &decision
			}
		}
	}
	if err = s.persistMaterializationBatch(ctx, candidates); err != nil {
		return nil, publicError(err)
	}
	for index := range candidates {
		completed[candidates[index].claimOrdinal] = true
	}
	return completed, nil
}

func (s *serviceImpl) readMaterializationBatch(ctx context.Context, claimed []notificationcap.ClaimedEvent) ([]materializationReadRow, error) {
	values := make([]string, len(claimed))
	arguments := make([]any, 0, len(claimed)*5)
	for index := range claimed {
		values[index] = "(?, ?, ?, ?, ?)"
		arguments = append(arguments, index, claimed[index].SourcePlugin, claimed[index].EventNo,
			claimed[index].Request.EventCode, string(claimed[index].Request.Recipient.Kind))
	}
	query := `
WITH requested(ordinal,source_plugin,source_event_no,event_code,recipient_kind) AS (
    VALUES ` + strings.Join(values, ",") + `
)
SELECT requested.ordinal,requested.source_plugin,requested.source_event_no,
       event.event_code,event.topic_code,event.object_type,event.required_variables,event.optional_variables,event.security_required,
       policy.recipient_kind,policy.channel_code,policy.enabled AS policy_enabled,policy.version AS policy_version,
       channel.status_code AS channel_status,coalesce(channel.provider_code,'') AS channel_provider_code,
       coalesce(template.template_no,'') AS template_no,coalesce(template.version,0) AS template_version,
       coalesce(template.locale_code,'') AS locale_code,coalesce(template.subject_template,'') AS subject_template,
       coalesce(template.body_template,'') AS body_template,coalesce(template.provider_code,'') AS template_provider_code,
       coalesce(template.provider_template_id,'') AS provider_template_id,
	   coalesce(template.provider_sms_variable_order,'[]'::jsonb)::text AS provider_sms_variable_order,
	   coalesce(template.provider_sms_data_mapping,'{}'::jsonb)::text AS provider_sms_data_mapping,
       coalesce(template.provider_wechat_data_mapping,'{}'::jsonb)::text AS provider_wechat_data_mapping,
       coalesce(materialization.materialization_no,'') AS existing_materialization_no
FROM requested
JOIN plugin_idc_notify_event_catalog event
  ON event.tenant_id=0 AND event.event_code=requested.event_code AND event.enabled=TRUE
JOIN plugin_idc_notify_event_policy policy
  ON policy.tenant_id=0 AND policy.event_code=requested.event_code AND policy.recipient_kind=requested.recipient_kind
JOIN plugin_idc_notify_channel channel
  ON channel.tenant_id=0 AND channel.channel_code=policy.channel_code
LEFT JOIN plugin_idc_notify_template_version template
  ON template.tenant_id=0 AND template.event_code=policy.event_code
 AND template.channel_code=policy.channel_code AND template.status_code='active'
LEFT JOIN plugin_idc_notify_event_materialization materialization
  ON materialization.tenant_id=0 AND materialization.source_plugin=requested.source_plugin
 AND materialization.source_event_no=requested.source_event_no AND materialization.channel_code=policy.channel_code
ORDER BY requested.ordinal,policy.channel_code,template.locale_code`
	var rows []materializationReadRow
	if err := s.database.GetScan(ctx, &rows, query, arguments...); err != nil {
		return nil, err
	}
	return rows, nil
}

func buildMaterializationCandidates(claimed []notificationcap.ClaimedEvent, rows []materializationReadRow) ([]materializationCandidate, []bool, error) {
	byKey := make(map[string]int)
	candidates := make([]materializationCandidate, 0, len(rows))
	seenClaim := make([]bool, len(claimed))
	for _, row := range rows {
		if row.Ordinal < 0 || row.Ordinal >= len(claimed) || row.SourceEventNo != claimed[row.Ordinal].EventNo ||
			row.SourcePlugin != claimed[row.Ordinal].SourcePlugin || row.EventCode != claimed[row.Ordinal].Request.EventCode ||
			row.RecipientKind != string(claimed[row.Ordinal].Request.Recipient.Kind) {
			return nil, nil, bizerr.NewCode(CodeInvalid)
		}
		seenClaim[row.Ordinal] = true
		key := fmt.Sprintf("%d\x00%s", row.Ordinal, row.ChannelCode)
		candidateIndex, exists := byKey[key]
		if !exists {
			candidateIndex = len(candidates)
			byKey[key] = candidateIndex
			candidates = append(candidates, materializationCandidate{
				claimOrdinal: row.Ordinal, claim: claimed[row.Ordinal], channel: row.ChannelCode,
				channelStatus: row.ChannelStatus, channelProviderCode: row.ChannelProviderCode,
				event: eventRow{EventCode: row.EventCode, TopicCode: row.TopicCode, ObjectType: row.ObjectType,
					RequiredVariables: row.RequiredVariables, OptionalVariables: row.OptionalVariables, SecurityRequired: row.SecurityRequired},
				policy: eventPolicyRow{EventCode: row.EventCode, TopicCode: row.TopicCode,
					RecipientKind: row.RecipientKind, ChannelCode: row.ChannelCode, Enabled: row.PolicyEnabled, Version: row.PolicyVersion},
				templates: make(map[string]templateRow), completed: row.ExistingMaterializationNo != "",
			})
		}
		if row.TemplateNo != "" {
			candidates[candidateIndex].templates[row.LocaleCode] = templateRow{TemplateNo: row.TemplateNo,
				Version: row.TemplateVersion, EventCode: row.EventCode, ChannelCode: row.ChannelCode,
				LocaleCode: row.LocaleCode, SubjectTemplate: row.SubjectTemplate, BodyTemplate: row.BodyTemplate,
				ProviderCode: row.TemplateProviderCode, ProviderTemplateID: row.ProviderTemplateID,
				ProviderSMSVariableOrder: row.ProviderSMSVariableOrder, ProviderSMSDataMapping: row.ProviderSMSDataMapping,
				ProviderWeChatDataMapping: row.ProviderWeChatDataMapping, StatusCode: "active"}
		}
	}
	for _, seen := range seenClaim {
		if !seen {
			return nil, nil, bizerr.NewCode(CodeInvalid)
		}
	}
	completed := make([]bool, len(claimed))
	for index := range candidates {
		if candidates[index].completed {
			completed[candidates[index].claimOrdinal] = true
		}
	}
	return candidates, completed, nil
}

func materializationContactRequests(candidates []materializationCandidate) ([][]int, []notificationcap.ContactRequest) {
	indexes := make([][]int, 0)
	requests := make([]notificationcap.ContactRequest, 0)
	byKey := make(map[string]int)
	for index := range candidates {
		candidate := &candidates[index]
		if candidate.completed {
			continue
		}
		if !candidate.policy.Enabled {
			candidate.outcome = materializationOutcome{status: "skipped", reason: "policy_disabled"}
			continue
		}
		if candidate.channel == "signed_webhook" {
			if ChannelStatus(candidate.channelStatus) != ChannelEnabled {
				candidate.outcome = materializationOutcome{status: "skipped", reason: "channel_unavailable"}
			}
			continue
		}
		switch candidate.claim.Request.Recipient.Kind {
		case notificationcap.RecipientStaffUser:
			candidate.outcome = materializationOutcome{status: "skipped", reason: "staff_resolver_unavailable"}
			continue
		case notificationcap.RecipientVerifiedDestination:
			candidate.outcome = materializationOutcome{status: "skipped", reason: "verified_destination_resolver_unavailable"}
			continue
		case notificationcap.RecipientCustomerContact:
		default:
			candidate.outcome = materializationOutcome{status: "failed", reason: "recipient_invalid"}
			continue
		}
		if ChannelStatus(candidate.channelStatus) != ChannelEnabled {
			candidate.outcome = materializationOutcome{status: "failed", reason: "channel_unavailable"}
			continue
		}
		recipient := candidate.claim.Request.Recipient
		request := notificationcap.ContactRequest{CustomerNo: recipient.CustomerNo, ContactNo: recipient.ContactNo,
			Channel: notificationcap.Channel(candidate.channel), Topic: notificationcap.Topic(candidate.event.TopicCode)}
		key := request.CustomerNo + "\x00" + request.ContactNo + "\x00" + string(request.Channel) + "\x00" + string(request.Topic)
		requestIndex, exists := byKey[key]
		if !exists {
			requestIndex = len(requests)
			byKey[key] = requestIndex
			requests = append(requests, request)
			indexes = append(indexes, nil)
		}
		indexes[requestIndex] = append(indexes[requestIndex], index)
	}
	return indexes, requests
}

func prepareMaterializationInputs(candidates []materializationCandidate) ([]int, []notificationcap.MarketingSendInput, error) {
	marketingIndexes := make([]int, 0)
	marketingInputs := make([]notificationcap.MarketingSendInput, 0)
	for index := range candidates {
		candidate := &candidates[index]
		if candidate.completed || candidate.outcome.status != "" {
			continue
		}
		request := candidate.claim.Request
		if candidate.event.ObjectType != string(request.BusinessObjectType) {
			candidate.outcome = materializationOutcome{status: "failed", reason: "event_invalid"}
			continue
		}
		if err := validateEventValues(request.Variables, &candidate.event); err != nil {
			candidate.outcome = materializationOutcome{status: "failed", reason: "event_variables_invalid"}
			continue
		}
		if candidate.channel == "signed_webhook" {
			candidate.outcome.input = &EnqueueInput{
				EventCode: request.EventCode, ChannelCode: candidate.channel, Locale: "en-US",
				BusinessObjectType: string(request.BusinessObjectType), BusinessObjectNo: request.BusinessObjectNo,
				BusinessObjectVersion: request.BusinessObjectVersion,
				IdempotencyKey:        stableNumber("nevent_", candidate.claim.SourcePlugin, candidate.claim.EventNo, candidate.channel),
			}
			continue
		}
		if !candidate.event.SecurityRequired && candidate.projection.PreferencePresent && !candidate.projection.PreferenceEnabled {
			candidate.outcome = materializationOutcome{status: "skipped", reason: "preference_blocked"}
			continue
		}
		locale := candidate.projection.PreferredLocale
		if !validLocale(locale) {
			candidate.outcome = materializationOutcome{status: "failed", reason: "locale_unavailable"}
			continue
		}
		template, exists := candidate.templates[locale]
		if !exists || !materializationTemplateReady(candidate.channel, candidate.channelProviderCode, template) {
			candidate.outcome = materializationOutcome{status: "failed", reason: "template_unavailable"}
			continue
		}
		subject, err := renderTemplate(template.SubjectTemplate, request.Variables)
		if err != nil {
			candidate.outcome = materializationOutcome{status: "failed", reason: "template_invalid"}
			continue
		}
		body, err := renderTemplate(template.BodyTemplate, request.Variables)
		if err != nil {
			candidate.outcome = materializationOutcome{status: "failed", reason: "template_invalid"}
			continue
		}
		input := EnqueueInput{
			EventCode: request.EventCode, ChannelCode: candidate.channel, Locale: locale,
			CustomerNo: request.Recipient.CustomerNo, ContactNo: request.Recipient.ContactNo,
			BusinessObjectType: string(request.BusinessObjectType), BusinessObjectNo: request.BusinessObjectNo,
			BusinessObjectVersion: request.BusinessObjectVersion, MarketingSnapshotNo: request.MarketingSnapshotNo,
			TemplateNo: template.TemplateNo, TemplateVersion: template.Version,
			Subject: subject, Body: body, Variables: cloneVariables(request.Variables),
			IdempotencyKey: stableNumber("nevent_", candidate.claim.SourcePlugin, candidate.claim.EventNo, candidate.channel),
		}
		if candidate.channel == string(notificationcap.ChannelInApp) {
			if candidate.projection.HostUserID <= 0 || !validInboxContent(subject, body) {
				candidate.outcome = materializationOutcome{status: "failed", reason: "recipient_unavailable"}
				continue
			}
			input.OwnerUserID = candidate.projection.HostUserID
		}
		candidate.outcome.input = &input
		if candidate.event.TopicCode != string(notificationcap.TopicMarketing) {
			continue
		}
		variablesJSON, err := jsonObject(input.Variables)
		if err != nil {
			candidate.outcome = materializationOutcome{status: "failed", reason: "event_variables_invalid"}
			continue
		}
		digest := digestText(input.EventCode, input.ChannelCode, input.Locale, input.Subject, input.Body, variablesJSON)
		marketingIndexes = append(marketingIndexes, index)
		marketingInputs = append(marketingInputs, notificationcap.MarketingSendInput{
			CampaignNo: request.BusinessObjectNo, SnapshotNo: request.MarketingSnapshotNo,
			CustomerNo: request.Recipient.CustomerNo, ContactNo: request.Recipient.ContactNo,
			Channel: notificationcap.Channel(candidate.channel), ContentDigest: digest,
		})
	}
	return marketingIndexes, marketingInputs, nil
}

func materializationTemplateReady(channel, activeProvider string, template templateRow) bool {
	if template.TemplateNo == "" || template.Version < 1 || template.StatusCode != "active" {
		return false
	}
	switch channel {
	case string(notificationcap.ChannelSMS):
		var order []string
		mapping := make(map[string]string)
		if template.ProviderCode != activeProvider || json.Unmarshal([]byte(template.ProviderSMSVariableOrder), &order) != nil ||
			len(order) > MaximumVariables || json.Unmarshal([]byte(template.ProviderSMSDataMapping), &mapping) != nil || len(mapping) > MaximumVariables {
			return false
		}
		switch template.ProviderCode {
		case providerTencentSMS:
			return numericTemplateID(template.ProviderTemplateID) && len(mapping) == 0
		case providerAliyunSMS:
			return providerTemplatePattern.MatchString(template.ProviderTemplateID) && len(order) == 0
		case providerSMSBao:
			return template.ProviderTemplateID == "" && len(order) == 0 && len(mapping) == 0
		default:
			return false
		}
	case string(notificationcap.ChannelWeChatOfficial):
		mapping := make(map[string]string)
		return template.ProviderTemplateID != "" &&
			json.Unmarshal([]byte(template.ProviderWeChatDataMapping), &mapping) == nil && len(mapping) > 0 && len(mapping) <= MaximumVariables
	default:
		return true
	}
}

func cloneVariables(values map[string]string) map[string]string {
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func validateEventValues(values map[string]string, event *eventRow) error {
	if event == nil || len(values) > MaximumVariables {
		return bizerr.NewCode(CodeInvalid)
	}
	allowed := eventVariableSet(event)
	for key, value := range values {
		if _, ok := allowed[key]; !ok || len(value) > 4096 {
			return bizerr.NewCode(CodeInvalid)
		}
	}
	var required []string
	if err := json.Unmarshal([]byte(event.RequiredVariables), &required); err != nil {
		return bizerr.WrapCode(err, CodeUnavailable)
	}
	for _, key := range required {
		if _, ok := values[key]; !ok {
			return bizerr.NewCode(CodeInvalid)
		}
	}
	return nil
}

func (s *serviceImpl) persistMaterializationBatch(ctx context.Context, candidates []materializationCandidate) error {
	chains := make([]modeldo.DeliveryChain, 0)
	tasks := make([]modeldo.DeliveryTask, 0)
	audits := make([]modeldo.DeliveryAudit, 0)
	ledgers := make([]modeldo.EventMaterialization, 0)
	active := make([]materializationCandidate, 0)
	for index := range candidates {
		candidate := &candidates[index]
		if candidate.completed {
			continue
		}
		outcome := candidate.outcome
		if outcome.status == "" {
			if outcome.input == nil {
				return bizerr.NewCode(CodeUnavailable)
			}
			outcome.status = "materialized"
			outcome.reason = "delivery_queued"
			candidate.outcome = outcome
		}
		materializationNo := stableNumber("nmat_", candidate.claim.SourcePlugin, candidate.claim.EventNo, candidate.channel)
		recipient := candidate.claim.Request.Recipient
		recipientDigest := digestText(string(recipient.Kind), recipient.CustomerNo, recipient.ContactNo,
			fmt.Sprintf("%d", recipient.StaffUserID), recipient.DestinationNo)
		ledger := modeldo.EventMaterialization{
			TenantId: notificationTenantID, MaterializationNo: materializationNo,
			SourcePlugin: candidate.claim.SourcePlugin, SourceEventNo: candidate.claim.EventNo,
			EventCode: candidate.claim.Request.EventCode, RecipientKind: string(recipient.Kind),
			RecipientDigest: recipientDigest, ChannelCode: candidate.channel,
			PolicyVersion: candidate.policy.Version, StatusCode: outcome.status, ReasonCode: outcome.reason,
		}
		if outcome.status == "materialized" {
			input := *outcome.input
			variablesJSON, err := jsonObject(input.Variables)
			if err != nil {
				return err
			}
			digest := digestText(input.EventCode, input.ChannelCode, input.Locale, input.Subject, input.Body, variablesJSON)
			taskNo := stableNumber("ntask_", input.EventCode, input.ChannelCode, input.IdempotencyKey)
			chainNo := stableNumber("nchain_", input.EventCode, input.ChannelCode, input.IdempotencyKey)
			ledger.TaskNo = taskNo
			chain := modeldo.DeliveryChain{TenantId: notificationTenantID, ChainNo: chainNo,
				RootTaskNo: taskNo, TailTaskNo: taskNo}
			templateNo, templateVersion := any(input.TemplateNo), any(input.TemplateVersion)
			recipientMask := "contact"
			if candidate.channel == "signed_webhook" {
				templateNo, templateVersion = nil, nil
				recipientMask = "configured_endpoint"
			}
			task := modeldo.DeliveryTask{
				TenantId: notificationTenantID, TaskNo: taskNo, ChainNo: chainNo,
				EventCode: input.EventCode, TopicCode: candidate.event.TopicCode, ChannelCode: input.ChannelCode,
				TemplateNo: templateNo, TemplateVersion: templateVersion, LocaleCode: input.Locale,
				CustomerNo: input.CustomerNo, ContactNo: input.ContactNo, RecipientMask: recipientMask,
				BusinessObjectType: input.BusinessObjectType, BusinessObjectNo: input.BusinessObjectNo,
				BusinessObjectVersion: input.BusinessObjectVersion, MarketingSnapshotNo: input.MarketingSnapshotNo,
				SubjectContent: input.Subject, BodyContent: input.Body, ProviderVariables: variablesJSON,
				ContentDigest: digest, StatusCode: string(DeliveryQueued), ResultClass: "none",
			}
			if candidate.marketing != nil {
				task.MarketingDecisionDigest = candidate.marketing.DecisionDigest
				task.MarketingConsentEventNo = candidate.marketing.ConsentEventNo
				task.MarketingConsentVersion = candidate.marketing.ConsentVersion
				task.MarketingPreferenceVersion = candidate.marketing.PreferenceVersion
				task.MarketingQuietHoursVersion = candidate.marketing.QuietHoursVersion
				task.MarketingEvaluatedAt = candidate.marketing.EvaluatedAt.UTC()
			}
			if input.OwnerUserID > 0 {
				task.OwnerUserId = input.OwnerUserID
			}
			chains = append(chains, chain)
			tasks = append(tasks, task)
			audits = append(audits, modeldo.DeliveryAudit{
				TenantId: notificationTenantID, AuditNo: stableNumber("nda_", taskNo, "enqueue", "1"),
				TaskNo: taskNo, ActionCode: "enqueue", ActorType: "system",
				ReasonCode: "business_event_policy", PreviousStatus: "", CurrentStatus: string(DeliveryQueued), TaskVersion: 1,
			})
			active = append(active, *candidate)
		}
		ledgers = append(ledgers, ledger)
	}
	if len(ledgers) == 0 {
		return nil
	}
	return s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if err := s.verifyMaterializationFences(ctx, active); err != nil {
			return err
		}
		if len(chains) > 0 {
			if _, err := s.database.Model(deliveryChainTable).Ctx(ctx).Data(chains).InsertIgnore(); err != nil {
				return err
			}
			if _, err := s.database.Model(deliveryTable).Ctx(ctx).Data(tasks).InsertIgnore(); err != nil {
				return err
			}
			if _, err := s.database.Model("plugin_idc_notify_delivery_audit").Ctx(ctx).Data(audits).InsertIgnore(); err != nil {
				return err
			}
		}
		_, err := s.database.Model(eventMaterializationTable).Ctx(ctx).Data(ledgers).InsertIgnore()
		return err
	})
}

func validInboxContent(subject, body string) bool {
	return subject != "" && body != "" && subject == strings.TrimSpace(subject) && body == strings.TrimSpace(body) &&
		utf8.RuneCountInString(subject) <= 200 && utf8.RuneCountInString(body) <= 4000
}

func (s *serviceImpl) verifyMaterializationFences(ctx context.Context, candidates []materializationCandidate) error {
	if len(candidates) == 0 {
		return nil
	}
	type fence struct {
		EventCode     string `orm:"event_code"`
		RecipientKind string `orm:"recipient_kind"`
		ChannelCode   string `orm:"channel_code"`
		PolicyEnabled bool   `orm:"policy_enabled"`
		PolicyVersion int64  `orm:"policy_version"`
		ChannelStatus string `orm:"channel_status"`
	}
	values := make([]string, len(candidates))
	arguments := make([]any, 0, len(candidates)*3)
	for index := range candidates {
		values[index] = "(?, ?, ?)"
		arguments = append(arguments, candidates[index].policy.EventCode,
			candidates[index].policy.RecipientKind, candidates[index].channel)
	}
	query := `
WITH requested(event_code,recipient_kind,channel_code) AS (VALUES ` + strings.Join(values, ",") + `)
SELECT policy.event_code,policy.recipient_kind,policy.channel_code,policy.enabled AS policy_enabled,
       policy.version AS policy_version,channel.status_code AS channel_status
FROM requested
JOIN plugin_idc_notify_event_policy policy
  ON policy.tenant_id=0 AND policy.event_code=requested.event_code
 AND policy.recipient_kind=requested.recipient_kind AND policy.channel_code=requested.channel_code
JOIN plugin_idc_notify_channel channel
  ON channel.tenant_id=0 AND channel.channel_code=policy.channel_code
ORDER BY policy.event_code,policy.recipient_kind,policy.channel_code
FOR UPDATE OF policy,channel`
	var rows []fence
	if err := s.database.GetScan(ctx, &rows, query, arguments...); err != nil {
		return err
	}
	if len(rows) != len(candidates) {
		return bizerr.NewCode(CodeConflict)
	}
	sort.Slice(candidates, func(i, j int) bool {
		left := candidates[i].policy.EventCode + "\x00" + candidates[i].policy.RecipientKind + "\x00" + candidates[i].channel
		right := candidates[j].policy.EventCode + "\x00" + candidates[j].policy.RecipientKind + "\x00" + candidates[j].channel
		return left < right
	})
	for index := range rows {
		candidate := candidates[index]
		row := rows[index]
		if row.EventCode != candidate.policy.EventCode || row.RecipientKind != candidate.policy.RecipientKind ||
			row.ChannelCode != candidate.channel || !row.PolicyEnabled || row.PolicyVersion != candidate.policy.Version ||
			ChannelStatus(row.ChannelStatus) != ChannelEnabled {
			return bizerr.NewCode(CodeConflict)
		}
	}
	return nil
}
