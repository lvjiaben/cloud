package notification

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/notificationcap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-notify/backend/cap/channelcap"
	modeldo "lina-plugin-idc-notify/backend/internal/model/do"
	"lina-plugin-shared-services/backend/cap/inboxcap"
)

const (
	deliveryTable        = "plugin_idc_notify_delivery_task"
	deliveryChainTable   = "plugin_idc_notify_delivery_chain"
	deliveryAttemptTable = "plugin_idc_notify_delivery_attempt"
	maximumWorkerBatch   = 50
	deliveryLease        = 2 * time.Minute
)

var retryDelays = [...]time.Duration{time.Minute, 5 * time.Minute, 30 * time.Minute}

type deliveryRow struct {
	TaskNo                      string     `orm:"task_no"`
	ChainNo                     string     `orm:"chain_no"`
	PreviousTaskNo              *string    `orm:"previous_task_no"`
	EventCode                   string     `orm:"event_code"`
	TopicCode                   string     `orm:"topic_code"`
	ChannelCode                 string     `orm:"channel_code"`
	TemplateNo                  *string    `orm:"template_no"`
	TemplateVersion             *int64     `orm:"template_version"`
	LocaleCode                  string     `orm:"locale_code"`
	OwnerUserID                 *int       `orm:"owner_user_id"`
	CustomerNo                  string     `orm:"customer_no"`
	ContactNo                   string     `orm:"contact_no"`
	RecipientMask               string     `orm:"recipient_mask"`
	BusinessObjectType          string     `orm:"business_object_type"`
	BusinessObjectNo            string     `orm:"business_object_no"`
	BusinessObjectVersion       int64      `orm:"business_object_version"`
	MarketingSnapshotNo         string     `orm:"marketing_snapshot_no"`
	SubjectContent              string     `orm:"subject_content"`
	BodyContent                 string     `orm:"body_content"`
	ProviderVariables           string     `orm:"provider_variables"`
	ContentDigest               string     `orm:"content_digest"`
	StatusCode                  string     `orm:"status_code"`
	ResultClass                 string     `orm:"result_class"`
	ExternalBoundaryCrossed     bool       `orm:"external_boundary_crossed"`
	ProviderMessageRef          *string    `orm:"provider_message_ref"`
	ProviderQueryDeadline       *time.Time `orm:"provider_query_deadline"`
	ProviderQueryDeadlineSource string     `orm:"provider_query_deadline_source"`
	RetrySequence               int        `orm:"retry_sequence"`
	NextAttemptAt               *time.Time `orm:"next_attempt_at"`
	Version                     int64      `orm:"version"`
	QueuedAt                    time.Time  `orm:"queued_at"`
	CompletedAt                 *time.Time `orm:"completed_at"`
	ManualResendCount           int        `orm:"manual_resend_count"`
	ChainTailTaskNo             string     `orm:"chain_tail_task_no"`
}

type deliveryChannelRow struct {
	ChannelCode            string  `orm:"channel_code"`
	ProviderCode           *string `orm:"provider_code"`
	StatusCode             string  `orm:"status_code"`
	BalanceState           string  `orm:"balance_state"`
	NonSecretConfiguration string  `orm:"non_secret_configuration"`
	CredentialSecretRef    *string `orm:"credential_secret_ref"`
}

type enqueueDecision struct {
	projection notificationcap.ContactProjection
	marketing  *notificationcap.MarketingSendDecision
}

func (s *serviceImpl) Enqueue(ctx context.Context, input EnqueueInput) (*Delivery, error) {
	if err := validateEnqueueInput(input); err != nil {
		return nil, err
	}
	event, err := s.loadEvent(ctx, input.EventCode)
	if err != nil || event == nil || event.ObjectType != input.BusinessObjectType {
		return nil, errorOrCode(err, CodeInvalid)
	}
	template, err := s.loadTemplate(ctx, input.TemplateNo, input.TemplateVersion)
	if err != nil || !validTemplateForEnqueue(template, input) {
		return nil, errorOrCode(err, CodeInvalid)
	}
	if err = validateEventValues(input.Variables, event); err != nil {
		return nil, err
	}
	channel, err := s.loadDeliveryChannel(ctx, input.ChannelCode)
	if err != nil || channel == nil || ChannelStatus(channel.StatusCode) != ChannelEnabled {
		return nil, errorOrCode(err, CodeChannelUnavailable)
	}
	if !deliveryProviderMatches(input.ChannelCode, channel, template) {
		return nil, bizerr.NewCode(CodeChannelUnavailable)
	}
	variablesJSON, err := jsonObject(input.Variables)
	if err != nil {
		return nil, err
	}
	digest := digestText(input.EventCode, input.ChannelCode, input.Locale, input.Subject, input.Body, variablesJSON)
	decision, err := s.evaluateRecipient(ctx, input, event, digest)
	if err != nil {
		return nil, err
	}
	taskNo := stableNumber("ntask_", input.EventCode, input.ChannelCode, input.IdempotencyKey)
	chainNo := stableNumber("nchain_", input.EventCode, input.ChannelCode, input.IdempotencyKey)
	if existing, loadErr := s.loadDelivery(ctx, taskNo); loadErr != nil {
		return nil, publicError(loadErr)
	} else if existing != nil {
		if existing.ContentDigest != digest || existing.ChainNo != chainNo {
			return nil, bizerr.NewCode(CodeConflict)
		}
		item := projectDelivery(*existing)
		return &item, nil
	}

	data := modeldo.DeliveryTask{
		TenantId: notificationTenantID, TaskNo: taskNo,
		ChainNo: chainNo, EventCode: input.EventCode,
		TopicCode: event.TopicCode, ChannelCode: input.ChannelCode,
		TemplateNo: input.TemplateNo, TemplateVersion: input.TemplateVersion,
		LocaleCode: input.Locale, CustomerNo: input.CustomerNo, ContactNo: input.ContactNo,
		RecipientMask:      safeRecipientMask(input, decision.projection),
		BusinessObjectType: input.BusinessObjectType, BusinessObjectNo: input.BusinessObjectNo,
		BusinessObjectVersion: input.BusinessObjectVersion, MarketingSnapshotNo: input.MarketingSnapshotNo,
		SubjectContent: input.Subject, BodyContent: input.Body, ProviderVariables: variablesJSON, ContentDigest: digest,
		StatusCode: string(DeliveryQueued), ResultClass: "none",
	}
	if input.OwnerUserID > 0 {
		data.OwnerUserId = input.OwnerUserID
	}
	if decision.marketing != nil {
		data.MarketingDecisionDigest = decision.marketing.DecisionDigest
		data.MarketingConsentEventNo = decision.marketing.ConsentEventNo
		data.MarketingConsentVersion = decision.marketing.ConsentVersion
		data.MarketingPreferenceVersion = decision.marketing.PreferenceVersion
		data.MarketingQuietHoursVersion = decision.marketing.QuietHoursVersion
		data.MarketingEvaluatedAt = decision.marketing.EvaluatedAt.UTC()
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var lockedChannel deliveryChannelRow
		if loadErr := s.database.Model(channelTable).Ctx(ctx).Where(tenantIDColumn, notificationTenantID).
			Where("channel_code", input.ChannelCode).LockUpdate().Scan(&lockedChannel); loadErr != nil {
			return loadErr
		}
		if lockedChannel.ChannelCode == "" || ChannelStatus(lockedChannel.StatusCode) != ChannelEnabled {
			return bizerr.NewCode(CodeChannelUnavailable)
		}
		if _, insertErr := s.database.Model(deliveryChainTable).Ctx(ctx).Data(modeldo.DeliveryChain{
			TenantId: notificationTenantID, ChainNo: chainNo,
			RootTaskNo: taskNo, TailTaskNo: taskNo,
		}).Insert(); insertErr != nil {
			return insertErr
		}
		if _, insertErr := s.database.Model(deliveryTable).Ctx(ctx).Data(data).Insert(); insertErr != nil {
			return insertErr
		}
		return s.appendDeliveryAudit(ctx, taskNo, "enqueue", "system", 0, "business_event", "", string(DeliveryQueued), 1)
	})
	if err != nil {
		if existing, loadErr := s.loadDelivery(ctx, taskNo); loadErr == nil && existing != nil && existing.ContentDigest == digest {
			item := projectDelivery(*existing)
			return &item, nil
		}
		return nil, publicError(err)
	}
	created, err := s.loadDelivery(ctx, taskNo)
	if err != nil || created == nil {
		return nil, publicError(errorOrCode(err, CodeUnavailable))
	}
	item := projectDelivery(*created)
	return &item, nil
}

func validTemplateForEnqueue(template *templateRow, input EnqueueInput) bool {
	return template != nil && template.EventCode == input.EventCode && template.ChannelCode == input.ChannelCode &&
		template.LocaleCode == input.Locale && template.StatusCode == "active"
}

func deliveryProviderMatches(channelCode string, channel *deliveryChannelRow, template *templateRow) bool {
	return channelCode != "sms" || channel != nil && channel.ProviderCode != nil &&
		template != nil && template.ProviderCode == *channel.ProviderCode
}

func (s *serviceImpl) ListDeliveries(ctx context.Context, input DeliveryListInput) (*DeliveryPage, error) {
	if _, err := s.authorize(ctx, "notification-deliveries"); err != nil {
		return nil, err
	}
	return s.listDeliveries(ctx, input, "")
}

func (s *serviceImpl) listDeliveries(ctx context.Context, input DeliveryListInput, customerNo string) (*DeliveryPage, error) {
	if input.Limit <= 0 {
		input.Limit = 20
	}
	if input.Limit > MaximumPageSize || input.ChannelCode != "" && !stableCodePattern.MatchString(input.ChannelCode) ||
		input.Status != "" && !validDeliveryStatus(input.Status) ||
		input.AfterTaskNo != "" && !stableCodePattern.MatchString(input.AfterTaskNo) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	model := s.database.Model(deliveryTable+" t").Ctx(ctx).
		LeftJoin(deliveryChainTable+" c", "c.tenant_id=t.tenant_id AND c.chain_no=t.chain_no").
		Where("t.tenant_id", notificationTenantID).
		Fields(deliveryFields())
	if customerNo != "" {
		model = model.Where("t.customer_no", customerNo)
	}
	if input.ChannelCode != "" {
		model = model.Where("t.channel_code", input.ChannelCode)
	}
	if input.Status != "" {
		model = model.Where("t.status_code", input.Status)
	}
	if input.AfterTaskNo != "" {
		var cursor deliveryRow
		cursorModel := s.database.Model(deliveryTable).Ctx(ctx).
			Where(tenantIDColumn, notificationTenantID).Where("task_no", input.AfterTaskNo)
		if customerNo != "" {
			cursorModel = cursorModel.Where("customer_no", customerNo)
		}
		if err := cursorModel.Fields("task_no,queued_at").Scan(&cursor); err != nil || cursor.TaskNo == "" {
			return nil, errorOrCode(err, CodeNotAccessible)
		}
		model = model.Where("(t.queued_at,t.task_no) < (?,?)", cursor.QueuedAt, cursor.TaskNo)
	}
	var rows []deliveryRow
	if err := model.OrderDesc("t.queued_at").OrderDesc("t.task_no").Limit(input.Limit + 1).Scan(&rows); err != nil {
		return nil, publicError(err)
	}
	page := &DeliveryPage{}
	if len(rows) > input.Limit {
		rows = rows[:input.Limit]
		page.NextCursor = rows[len(rows)-1].TaskNo
	}
	page.Items = make([]Delivery, len(rows))
	for index := range rows {
		page.Items[index] = projectDelivery(rows[index])
	}
	return page, nil
}

func (s *serviceImpl) GetDelivery(ctx context.Context, taskNo string) (*Delivery, error) {
	if _, err := s.authorize(ctx, "notification-delivery-"+taskNo); err != nil {
		return nil, err
	}
	if !stableCodePattern.MatchString(taskNo) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	row, err := s.loadDelivery(ctx, taskNo)
	if err != nil || row == nil {
		return nil, errorOrCode(err, CodeNotAccessible)
	}
	if err = s.authorizeBusinessObject(ctx, row); err != nil {
		return nil, err
	}
	item := projectDelivery(*row)
	return &item, nil
}

func (s *serviceImpl) ProcessDue(ctx context.Context, limit int) error {
	if limit <= 0 || limit > maximumWorkerBatch {
		return bizerr.NewCode(CodeInvalid)
	}
	rows, err := s.claimDue(ctx, limit)
	if err != nil {
		return publicError(err)
	}
	for index := range rows {
		if err = s.processClaimed(ctx, &rows[index]); err != nil {
			return publicError(err)
		}
	}
	return nil
}

func (s *serviceImpl) claimDue(ctx context.Context, limit int) ([]deliveryRow, error) {
	leaseID := stableNumber("nlease_", strconv.FormatInt(s.now().UTC().UnixNano(), 10))
	var rows []deliveryRow
	err := s.database.GetScan(ctx, &rows, `
WITH due AS (
    SELECT tenant_id,task_no
    FROM plugin_idc_notify_delivery_task
    WHERE tenant_id=? AND ((status_code IN ('queued','retry_wait') AND COALESCE(next_attempt_at,CURRENT_TIMESTAMP) <= CURRENT_TIMESTAMP)
        OR (status_code='sending' AND attempt_lease_until < CURRENT_TIMESTAMP))
    ORDER BY COALESCE(next_attempt_at,queued_at),task_no
    FOR UPDATE SKIP LOCKED
    LIMIT ?
)
UPDATE plugin_idc_notify_delivery_task t
SET status_code='sending',attempt_lease=?,attempt_lease_until=CURRENT_TIMESTAMP + INTERVAL '2 minutes',
    updated_at=CURRENT_TIMESTAMP,version=version+1
FROM due
WHERE t.tenant_id=due.tenant_id AND t.task_no=due.task_no
RETURNING t.*`, notificationTenantID, limit, leaseID)
	return rows, err
}

func (s *serviceImpl) processClaimed(ctx context.Context, task *deliveryRow) error {
	if task == nil {
		return nil
	}
	if !validChannelCode(task.ChannelCode) && !staffInboxTask(task) {
		return s.finishClaim(ctx, task, channelcap.Result{Class: "channel_disabled"}, DeliveryCancelled, "channel_removed")
	}
	channel, err := s.loadDeliveryChannel(ctx, task.ChannelCode)
	if err != nil {
		return err
	}
	if channel == nil || ChannelStatus(channel.StatusCode) != ChannelEnabled {
		return s.finishClaim(ctx, task, channelcap.Result{Class: "channel_disabled"}, DeliveryCancelled, "channel_disabled")
	}
	input := EnqueueInput{
		EventCode: task.EventCode, ChannelCode: task.ChannelCode, Locale: task.LocaleCode,
		CustomerNo: task.CustomerNo, ContactNo: task.ContactNo, BusinessObjectType: task.BusinessObjectType,
		BusinessObjectNo: task.BusinessObjectNo, MarketingSnapshotNo: task.MarketingSnapshotNo,
		Subject: task.SubjectContent, Body: task.BodyContent,
	}
	if task.TopicCode == string(notificationcap.TopicMarketing) {
		if _, gateErr := s.evaluateMarketing(ctx, input, task.ContentDigest); gateErr != nil {
			return s.finishClaim(ctx, task, channelcap.Result{Class: "unsubscribed"}, DeliveryPreferenceBlocked, "marketing_gate_blocked")
		}
	}
	if task.ChannelCode == string(notificationcap.ChannelInApp) {
		return s.deliverInApp(ctx, task)
	}
	if !customerAddressChannel(task.ChannelCode) {
		return s.deliverExternal(ctx, task, channel, nil)
	}
	event, err := s.loadEvent(ctx, task.EventCode)
	if err != nil || event == nil {
		return s.finishClaim(ctx, task, channelcap.Result{Class: channelcap.ResultConfigurationError}, DeliveryTemplateError, "event_unavailable")
	}
	called := false
	request := notificationcap.ContactRequest{CustomerNo: task.CustomerNo, ContactNo: task.ContactNo, Channel: notificationcap.Channel(task.ChannelCode), Topic: notificationcap.Topic(task.TopicCode)}
	err = s.sources.Contacts.ResolveBatch(ctx, []notificationcap.ContactRequest{request}, func(_ int, projection notificationcap.ContactProjection, destination []byte) error {
		called = true
		if projection.CustomerNo != task.CustomerNo || projection.ContactNo != task.ContactNo || len(destination) == 0 || len(destination) > 1024 {
			return s.finishClaim(ctx, task, channelcap.Result{Class: channelcap.ResultInvalidRecipient}, DeliveryFailed, "invalid_recipient")
		}
		if !event.SecurityRequired && projection.PreferencePresent && !projection.PreferenceEnabled {
			return s.finishClaim(ctx, task, channelcap.Result{Class: "unsubscribed"}, DeliveryPreferenceBlocked, "preference_blocked")
		}
		return s.deliverExternal(ctx, task, channel, destination)
	})
	if err != nil {
		return err
	}
	if !called {
		return s.finishClaim(ctx, task, channelcap.Result{Class: channelcap.ResultInvalidRecipient}, DeliveryFailed, "recipient_unavailable")
	}
	return nil
}

func (s *serviceImpl) deliverInApp(ctx context.Context, task *deliveryRow) error {
	if task.OwnerUserID == nil || *task.OwnerUserID <= 0 {
		return s.finishClaim(ctx, task, channelcap.Result{Class: channelcap.ResultInvalidRecipient}, DeliveryFailed, "recipient_unavailable")
	}
	_, err := s.beginAttempt(ctx, task, false)
	if err != nil {
		return err
	}
	result, err := s.inbox.Publish(ctx, inboxcap.PublishRequest{
		RecipientUserID: int64(*task.OwnerUserID), SourceType: "notification_delivery", SourceNo: task.TaskNo,
		IdempotencyKey: task.TaskNo + ":" + strconv.Itoa(task.RetrySequence),
		Title:          task.SubjectContent, Body: task.BodyContent, Category: task.TopicCode,
	})
	if err != nil {
		// Publication is owner-side atomic and idempotent. Leave this task under
		// its lease so a later worker can safely repeat the exact same request.
		return err
	}
	if result == nil || result.MessageNo == "" || result.EvidenceNo == "" {
		return bizerr.NewCode(CodeUnavailable)
	}
	return s.finishClaim(ctx, task, channelcap.Result{Class: channelcap.ResultDelivered,
		ProviderMessageRef: result.MessageNo, EvidenceDigest: digestText("shared_inbox", result.MessageNo, result.EvidenceNo)},
		DeliveryDelivered, "shared_inbox_published")
}

func (s *serviceImpl) deliverExternal(ctx context.Context, task *deliveryRow, channel *deliveryChannelRow, destination []byte) error {
	if channel == nil || channel.ProviderCode == nil || channel.CredentialSecretRef == nil {
		return s.finishClaim(ctx, task, channelcap.Result{Class: channelcap.ResultConfigurationError}, DeliveryFailed, "configuration_unavailable")
	}
	binding, exists := s.adapters[*channel.ProviderCode]
	if !exists {
		return s.finishClaim(ctx, task, channelcap.Result{Class: channelcap.ResultConfigurationError}, DeliveryFailed, "adapter_unavailable")
	}
	handle, err := secretcap.NewHandle(*channel.CredentialSecretRef)
	if err != nil {
		return s.finishClaim(ctx, task, channelcap.Result{Class: channelcap.ResultConfigurationError}, DeliveryFailed, "credential_unavailable")
	}
	configuration := make(map[string]string)
	if json.Unmarshal([]byte(channel.NonSecretConfiguration), &configuration) != nil {
		return s.finishClaim(ctx, task, channelcap.Result{Class: channelcap.ResultConfigurationError}, DeliveryFailed, "configuration_unavailable")
	}
	providerTemplateID, providerParameters, providerData, mappingErr := s.materializeProviderMapping(ctx, task, *channel.ProviderCode)
	if mappingErr != nil {
		return s.finishClaim(ctx, task, channelcap.Result{Class: channelcap.ResultConfigurationError}, DeliveryTemplateError, "provider_mapping_unavailable")
	}
	started, err := s.beginAttempt(ctx, task, true)
	if err != nil {
		return err
	}
	if !started {
		return s.finishClaim(ctx, task, channelcap.Result{Class: channelcap.ResultUnknown, CrossedExternalBorder: true}, DeliveryResultUnknown, "attempt_fence_recovered")
	}
	result, deliveryErr := binding.adapter.Deliver(ctx, channelcap.Delivery{
		TaskNo: task.TaskNo, IdempotencyKey: task.TaskNo + ":" + strconv.Itoa(task.RetrySequence),
		EventNo: task.ChainNo, EventType: task.EventCode,
		EventVersion: task.BusinessObjectVersion, OccurredAt: task.QueuedAt.UTC(),
		Credential: handle, Destination: destination, Subject: task.SubjectContent, Body: task.BodyContent,
		Locale: task.LocaleCode, ProviderTemplateID: providerTemplateID,
		ProviderParameters: providerParameters, ProviderData: providerData, NonSecretConfig: configuration,
	})
	if deliveryErr != nil {
		result = channelcap.Result{Class: channelcap.ResultUnknown, CrossedExternalBorder: true, EvidenceDigest: digestText("adapter_error", task.TaskNo, strconv.Itoa(task.RetrySequence))}
	}
	status := deliveryStatusForResult(result, task.RetrySequence)
	return s.finishClaim(ctx, task, result, status, "adapter_result")
}

func (s *serviceImpl) beginAttempt(ctx context.Context, task *deliveryRow, crossed bool) (bool, error) {
	attemptNo := stableNumber("natt_", task.TaskNo, strconv.Itoa(task.RetrySequence))
	fence := task.TaskNo + ":" + strconv.Itoa(task.RetrySequence)
	result, err := s.database.Model(deliveryAttemptTable).Ctx(ctx).Data(modeldo.DeliveryAttempt{
		TenantId: notificationTenantID, AttemptNo: attemptNo,
		TaskNo: task.TaskNo, RetrySequence: task.RetrySequence,
		AttemptFence: fence, CrossedExternalBoundary: crossed, ResultClass: "none",
		EvidenceDigest: digestText("attempt_started", fence), StartedAt: gdb.Raw("CURRENT_TIMESTAMP"),
	}).InsertIgnore()
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	return err == nil && rows == 1, err
}

func (s *serviceImpl) finishClaim(ctx context.Context, task *deliveryRow, result channelcap.Result, status DeliveryStatus, reason string) error {
	if !isDigest(result.EvidenceDigest) {
		result.EvidenceDigest = digestText(task.TaskNo, strconv.Itoa(task.RetrySequence), string(result.Class), reason)
	}
	if (status == DeliverySubmitted || status == DeliveryResultUnknown) && result.QueryDeadline == nil {
		status = DeliveryManualReview
		reason = "query_authority_unavailable"
	}
	data := modeldo.DeliveryTask{
		StatusCode: string(status), ResultClass: string(result.Class),
		ExternalBoundaryCrossed:     result.CrossedExternalBorder || task.ExternalBoundaryCrossed,
		ProviderMessageRef:          nullableString(result.ProviderMessageRef),
		ProviderQueryDeadline:       nullableTimeValue(result.QueryDeadline),
		ProviderQueryDeadlineSource: result.QueryDeadlineSource,
		AttemptLease:                gdb.Raw("NULL"), AttemptLeaseUntil: gdb.Raw("NULL"),
		Version: gdb.Raw("version + 1"),
	}
	if status == DeliveryRetryWait {
		nextSequence := task.RetrySequence + 1
		data.RetrySequence = nextSequence
		data.NextAttemptAt = gdb.Raw("CURRENT_TIMESTAMP + INTERVAL '" + retryIntervalLiteral(nextSequence) + "'")
	} else {
		data.NextAttemptAt = gdb.Raw("NULL")
		if isTerminalDeliveryStatus(status) {
			data.CompletedAt = gdb.Raw("CURRENT_TIMESTAMP")
		}
	}
	return s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if _, err := s.database.Model(deliveryAttemptTable).Ctx(ctx).
			Where(tenantIDColumn, notificationTenantID).Where("task_no", task.TaskNo).
			Where("retry_sequence", task.RetrySequence).
			Data(modeldo.DeliveryAttempt{CrossedExternalBoundary: result.CrossedExternalBorder || task.ExternalBoundaryCrossed,
				ResultClass: string(result.Class), EvidenceDigest: result.EvidenceDigest,
				ProviderReference: nullableString(result.ProviderMessageRef), FinishedAt: gdb.Raw("CURRENT_TIMESTAMP")}).Update(); err != nil {
			return err
		}
		updated, err := s.database.Model(deliveryTable).Ctx(ctx).
			Where(tenantIDColumn, notificationTenantID).Where("task_no", task.TaskNo).
			Where("status_code", string(DeliverySending)).Where("retry_sequence", task.RetrySequence).Data(data).Update()
		if err != nil || requireOne(updated) != nil {
			return bizerr.NewCode(CodeConflict)
		}
		return s.appendDeliveryAudit(ctx, task.TaskNo, "delivery_result", "system", 0, reason,
			string(DeliverySending), string(status), task.Version+1)
	})
}

func (s *serviceImpl) evaluateRecipient(ctx context.Context, input EnqueueInput, event *eventRow, digest string) (enqueueDecision, error) {
	decision := enqueueDecision{}
	if input.ChannelCode == string(notificationcap.ChannelInApp) {
		if input.OwnerUserID <= 0 {
			return decision, bizerr.NewCode(CodeInvalid)
		}
	} else if customerAddressChannel(input.ChannelCode) {
		request := notificationcap.ContactRequest{CustomerNo: input.CustomerNo, ContactNo: input.ContactNo, Channel: notificationcap.Channel(input.ChannelCode), Topic: notificationcap.Topic(event.TopicCode)}
		called := false
		err := s.sources.Contacts.ResolveBatch(ctx, []notificationcap.ContactRequest{request}, func(_ int, projection notificationcap.ContactProjection, destination []byte) error {
			called = true
			if projection.CustomerNo != input.CustomerNo || projection.ContactNo != input.ContactNo || len(destination) == 0 {
				return bizerr.NewCode(CodeChannelUnavailable)
			}
			decision.projection = projection
			return nil
		})
		if err != nil || !called {
			return decision, errorOrCode(err, CodeChannelUnavailable)
		}
		if !event.SecurityRequired && decision.projection.PreferencePresent && !decision.projection.PreferenceEnabled {
			return decision, bizerr.NewCode(CodeChannelUnavailable)
		}
	}
	if event.TopicCode == string(notificationcap.TopicMarketing) {
		marketing, err := s.evaluateMarketing(ctx, input, digest)
		if err != nil {
			return decision, err
		}
		decision.marketing = marketing
	}
	return decision, nil
}

func (s *serviceImpl) evaluateMarketing(ctx context.Context, input EnqueueInput, digest string) (*notificationcap.MarketingSendDecision, error) {
	if input.BusinessObjectType != string(notificationcap.BusinessObjectMarketingCampaign) || input.MarketingSnapshotNo == "" {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	request := notificationcap.MarketingSendInput{
		CampaignNo: input.BusinessObjectNo, SnapshotNo: input.MarketingSnapshotNo,
		CustomerNo: input.CustomerNo, ContactNo: input.ContactNo,
		Channel: notificationcap.Channel(input.ChannelCode), ContentDigest: digest,
	}
	items, err := s.sources.Marketing.CheckSendBatch(ctx, []notificationcap.MarketingSendInput{request})
	if err != nil || len(items) != 1 || !items[0].Allowed || items[0].DecisionDigest == "" ||
		items[0].CampaignNo != request.CampaignNo || items[0].SnapshotNo != request.SnapshotNo ||
		items[0].CustomerNo != request.CustomerNo || items[0].ContactNo != request.ContactNo ||
		items[0].Channel != request.Channel {
		return nil, errorOrCode(err, CodeChannelUnavailable)
	}
	return &items[0], nil
}

func (s *serviceImpl) loadDelivery(ctx context.Context, taskNo string) (*deliveryRow, error) {
	var row deliveryRow
	err := s.database.Model(deliveryTable+" t").Ctx(ctx).
		LeftJoin(deliveryChainTable+" c", "c.tenant_id=t.tenant_id AND c.chain_no=t.chain_no").
		Fields(deliveryFields()).Where("t.tenant_id", notificationTenantID).
		Where("t.task_no", taskNo).Scan(&row)
	if err != nil || row.TaskNo == "" {
		return nil, err
	}
	return &row, nil
}

func (s *serviceImpl) loadDeliveryChannel(ctx context.Context, code string) (*deliveryChannelRow, error) {
	var row deliveryChannelRow
	err := s.database.Model(channelTable).Ctx(ctx).Where(tenantIDColumn, notificationTenantID).
		Where("channel_code", code).
		Fields("channel_code,provider_code,status_code,balance_state,non_secret_configuration,credential_secret_ref").Scan(&row)
	if err != nil || row.ChannelCode == "" {
		return nil, err
	}
	return &row, nil
}

func deliveryFields() string {
	return `t.task_no,t.chain_no,t.previous_task_no,t.event_code,t.topic_code,t.channel_code,
        t.template_no,t.template_version,t.locale_code,t.owner_user_id,t.customer_no,t.contact_no,
        t.recipient_mask,t.business_object_type,t.business_object_no,t.business_object_version,
        t.marketing_snapshot_no,t.subject_content,t.body_content,t.provider_variables,t.content_digest,t.status_code,
        t.result_class,t.external_boundary_crossed,t.provider_message_ref,t.provider_query_deadline,
        t.provider_query_deadline_source,t.retry_sequence,t.next_attempt_at,t.version,t.queued_at,
        t.completed_at,COALESCE(c.manual_resend_count,0) AS manual_resend_count,
        COALESCE(c.tail_task_no,'') AS chain_tail_task_no`
}

func projectDelivery(row deliveryRow) Delivery {
	previous := ""
	if row.PreviousTaskNo != nil {
		previous = *row.PreviousTaskNo
	}
	return Delivery{
		TaskNo: row.TaskNo, ChainNo: row.ChainNo, PreviousTaskNo: previous,
		EventCode: row.EventCode, Topic: row.TopicCode, ChannelCode: row.ChannelCode,
		Locale: row.LocaleCode, RecipientMask: row.RecipientMask,
		BusinessObjectType: row.BusinessObjectType, BusinessObjectNo: row.BusinessObjectNo,
		Status: DeliveryStatus(row.StatusCode), ResultClass: row.ResultClass,
		RetrySequence: row.RetrySequence, NextAttemptAt: utcTimePointer(row.NextAttemptAt),
		QueryDeadline: utcTimePointer(row.ProviderQueryDeadline), QueryDeadlineSource: row.ProviderQueryDeadlineSource,
		ManualResendCount: row.ManualResendCount, IsChainTail: row.TaskNo == row.ChainTailTaskNo,
		Version: row.Version, QueuedAt: row.QueuedAt.UTC(), CompletedAt: utcTimePointer(row.CompletedAt),
	}
}

func validateEnqueueInput(input EnqueueInput) error {
	if !stableCodePattern.MatchString(input.EventCode) || !validChannelCode(input.ChannelCode) ||
		!validLocale(input.Locale) || !stableCodePattern.MatchString(input.BusinessObjectType) ||
		input.BusinessObjectNo == "" || len(input.BusinessObjectNo) > 128 || input.BusinessObjectVersion < 0 ||
		!stableCodePattern.MatchString(input.TemplateNo) || input.TemplateVersion <= 0 ||
		len(input.Subject)+len(input.Body) == 0 || len(input.Subject)+len(input.Body) > MaximumTemplateBytes ||
		!validateIdempotencyKey(input.IdempotencyKey) || input.OwnerUserID < 0 {
		return bizerr.NewCode(CodeInvalid)
	}
	if customerAddressChannel(input.ChannelCode) &&
		(input.CustomerNo == "" || input.ContactNo == "" || len(input.CustomerNo) > 80 || len(input.ContactNo) > 80) {
		return bizerr.NewCode(CodeInvalid)
	}
	return nil
}

func validDeliveryStatus(value string) bool {
	switch DeliveryStatus(value) {
	case DeliveryQueued, DeliverySending, DeliveryRetryWait, DeliverySubmitted, DeliveryDelivered,
		DeliverySucceeded, DeliveryFailed, DeliveryResultUnknown, DeliveryManualReview,
		DeliveryManualUnknownClosed, DeliveryCancelled, DeliveryTemplateError, DeliveryPreferenceBlocked:
		return true
	default:
		return false
	}
}

func deliveryStatusForResult(result channelcap.Result, sequence int) DeliveryStatus {
	switch result.Class {
	case channelcap.ResultDelivered:
		return DeliveryDelivered
	case channelcap.ResultAccepted:
		return DeliverySubmitted
	case channelcap.ResultDefinitiveNotReceivedTemporary:
		if sequence < len(retryDelays) {
			return DeliveryRetryWait
		}
		return DeliveryFailed
	case channelcap.ResultUnknown:
		return DeliveryResultUnknown
	default:
		return DeliveryFailed
	}
}

func retryIntervalLiteral(sequence int) string {
	switch sequence {
	case 1:
		return "1 minute"
	case 2:
		return "5 minutes"
	default:
		return "30 minutes"
	}
}

func isTerminalDeliveryStatus(status DeliveryStatus) bool {
	switch status {
	case DeliveryDelivered, DeliverySucceeded, DeliveryFailed, DeliveryManualUnknownClosed,
		DeliveryCancelled, DeliveryTemplateError, DeliveryPreferenceBlocked:
		return true
	default:
		return false
	}
}

func safeRecipientMask(input EnqueueInput, projection notificationcap.ContactProjection) string {
	if input.ChannelCode == string(notificationcap.ChannelInApp) {
		return "account"
	}
	if !customerAddressChannel(input.ChannelCode) {
		return "configured_endpoint"
	}
	if projection.ContactNo != "" {
		return "contact"
	}
	return ""
}

func customerAddressChannel(channelCode string) bool {
	switch notificationcap.Channel(channelCode) {
	case notificationcap.ChannelEmail, notificationcap.ChannelSMS:
		return true
	default:
		return false
	}
}

func (s *serviceImpl) materializeProviderMapping(ctx context.Context, task *deliveryRow, activeProvider string) (string, []string, map[string]string, error) {
	if task == nil {
		return "", nil, nil, bizerr.NewCode(CodeInvalid)
	}
	if task.ChannelCode != "sms" {
		return "", nil, nil, nil
	}
	if task.TemplateNo == nil || task.TemplateVersion == nil {
		return "", nil, nil, bizerr.NewCode(CodeInvalid)
	}
	template, err := s.loadTemplate(ctx, *task.TemplateNo, *task.TemplateVersion)
	if err != nil || template == nil || template.EventCode != task.EventCode || template.ChannelCode != task.ChannelCode || template.LocaleCode != task.LocaleCode {
		return "", nil, nil, errorOrCode(err, CodeInvalid)
	}
	variables := make(map[string]string)
	if json.Unmarshal([]byte(task.ProviderVariables), &variables) != nil {
		return "", nil, nil, bizerr.NewCode(CodeInvalid)
	}
	if template.ProviderCode != activeProvider {
		return "", nil, nil, bizerr.NewCode(CodeChannelUnavailable)
	}
	return materializeSMSProviderMapping(template, variables)
}

func nullableString(value string) any {
	if value == "" {
		return gdb.Raw("NULL")
	}
	return value
}

func nullableTimeValue(value *time.Time) any {
	if value == nil {
		return gdb.Raw("NULL")
	}
	return utcTimePointer(value)
}

func utcTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	utc := value.UTC()
	return &utc
}

func (s *serviceImpl) appendDeliveryAudit(ctx context.Context, taskNo, action, actorType string, actor int, reason, previous, current string, version int64) error {
	data := modeldo.DeliveryAudit{
		TenantId: notificationTenantID,
		AuditNo:  stableNumber("nda_", taskNo, action, strconv.FormatInt(version, 10)),
		TaskNo:   taskNo, ActionCode: action, ActorType: actorType,
		ReasonCode: reason, PreviousStatus: previous, CurrentStatus: current,
		TaskVersion: version,
	}
	if actor > 0 {
		data.ActorUserId = actor
	}
	_, err := s.database.Model("plugin_idc_notify_delivery_audit").Ctx(ctx).Data(data).InsertIgnore()
	return err
}

// staffInboxTask preserves the internal employee renewal inbox delivery only.
func staffInboxTask(task *deliveryRow) bool {
	return task != nil && task.ChannelCode == string(notificationcap.ChannelInApp) && task.EventCode == accessoryRenewalAttentionEventCode && task.OwnerUserID != nil && *task.OwnerUserID > 0
}
