package notification

import (
	"context"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/notificationcap"
	modeldo "lina-plugin-idc-notify/backend/internal/model/do"
)

const (
	maximumManualResends = 3
	manualResendCooldown = 10 * time.Minute
)

type chainRow struct {
	ChainNo            string     `orm:"chain_no"`
	RootTaskNo         string     `orm:"root_task_no"`
	TailTaskNo         string     `orm:"tail_task_no"`
	ManualResendCount  int        `orm:"manual_resend_count"`
	LastManualResendAt *time.Time `orm:"last_manual_resend_at"`
	Version            int64      `orm:"version"`
}

func (s *serviceImpl) GetResendEligibility(ctx context.Context, taskNo string) (*ResendEligibility, error) {
	actor, err := s.authorize(ctx, "notification-delivery-"+taskNo)
	if err != nil {
		return nil, err
	}
	if !stableCodePattern.MatchString(taskNo) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	task, err := s.loadDelivery(ctx, taskNo)
	if err != nil || task == nil {
		return nil, errorOrCode(err, CodeNotAccessible)
	}
	if err = s.authorizeBusinessObject(ctx, task); err != nil {
		return nil, err
	}
	now, err := s.databaseTime(ctx)
	if err != nil {
		return nil, publicError(err)
	}
	projection := &ResendEligibility{
		TaskNo: task.TaskNo, TaskVersion: task.Version, EvaluatedAt: now,
		RemainingCount: maximumManualResends - task.ManualResendCount,
	}
	if projection.RemainingCount < 0 {
		projection.RemainingCount = 0
	}
	unsafe := false
	if task.TaskNo != task.ChainTailTaskNo || task.StatusCode != string(DeliveryFailed) ||
		!explicitNotReceived(task.ResultClass) {
		projection.Reasons = append(projection.Reasons, ResendReasonTaskChanged)
		if task.TaskNo != task.ChainTailTaskNo {
			projection.ReplacedByTaskNo = task.ChainTailTaskNo
		}
	}
	channel, channelErr := s.loadDeliveryChannel(ctx, task.ChannelCode)
	if channelErr != nil || channel == nil {
		unsafe = true
	} else {
		if ChannelStatus(channel.StatusCode) != ChannelEnabled {
			projection.Reasons = append(projection.Reasons, ResendReasonChannelDisabled)
		}
		if channel.BalanceState != "sufficient" {
			projection.Reasons = append(projection.Reasons, ResendReasonInsufficientBalance)
		}
		if task.ChannelCode != string(notificationcap.ChannelInApp) &&
			(channel.ProviderCode == nil || channel.CredentialSecretRef == nil) {
			unsafe = true
		}
	}
	unsubscribed, recipientUnsafe := s.recipientCurrentlyBlocked(ctx, task)
	if unsubscribed {
		projection.Reasons = append(projection.Reasons, ResendReasonRecipientUnsubscribed)
	}
	unsafe = unsafe || recipientUnsafe
	if task.ManualResendCount >= maximumManualResends {
		projection.Reasons = append(projection.Reasons, ResendReasonLimitReached)
	}
	var chain chainRow
	if err = s.database.Model(deliveryChainTable).Ctx(ctx).Where(tenantIDColumn, notificationTenantID).
		Where("chain_no", task.ChainNo).Scan(&chain); err != nil || chain.ChainNo == "" {
		unsafe = true
	} else if chain.LastManualResendAt != nil {
		next := chain.LastManualResendAt.UTC().Add(manualResendCooldown)
		if now.Before(next) {
			projection.NextAllowedAt = &next
			projection.Reasons = append(projection.Reasons, ResendReasonCooldown)
		}
	}
	if err = s.requireRecentLogin(ctx, actor); err != nil {
		projection.Reasons = append(projection.Reasons, ResendReasonRecentLoginRequired)
	}
	if unsafe {
		projection.Reasons = append(projection.Reasons, ResendReasonUnsafe)
	}
	projection.Allowed = len(projection.Reasons) == 0
	return projection, nil
}

func (s *serviceImpl) ManualResend(ctx context.Context, input ManualResendInput) (*Delivery, error) {
	if !stableCodePattern.MatchString(input.TaskNo) || input.ExpectedVersion <= 0 ||
		!validateIdempotencyKey(input.OperationNo) || !input.Confirmed ||
		strings.TrimSpace(input.Reason) == "" || len(input.Reason) > MaximumReasonBytes {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	actor, err := s.authorize(ctx, "notification-delivery-"+input.TaskNo)
	if err != nil {
		return nil, err
	}
	if err = s.requireRecentLogin(ctx, actor); err != nil {
		return nil, err
	}
	if replay, replayErr := s.loadManualResendReplay(ctx, input.OperationNo, input.TaskNo); replayErr != nil || replay != "" {
		if replayErr != nil {
			return nil, publicError(replayErr)
		}
		created, loadErr := s.loadDelivery(ctx, replay)
		if loadErr != nil || created == nil {
			return nil, publicError(errorOrCode(loadErr, CodeUnavailable))
		}
		item := projectDelivery(*created)
		return &item, nil
	}
	task, err := s.loadDelivery(ctx, input.TaskNo)
	if err != nil || task == nil {
		return nil, errorOrCode(err, CodeNotAccessible)
	}
	if err = s.authorizeBusinessObject(ctx, task); err != nil {
		return nil, err
	}
	blocked, unsafe := s.recipientCurrentlyBlocked(ctx, task)
	if blocked || unsafe {
		return nil, bizerr.NewCode(CodeConflict)
	}
	newTaskNo := stableNumber("ntask_", task.ChainNo, input.OperationNo)
	err = s.createManualResend(ctx, input, task, actor, newTaskNo)
	if err != nil {
		if replay, replayErr := s.loadManualResendReplay(ctx, input.OperationNo, input.TaskNo); replayErr == nil && replay != "" {
			created, loadErr := s.loadDelivery(ctx, replay)
			if loadErr == nil && created != nil {
				item := projectDelivery(*created)
				return &item, nil
			}
		}
		return nil, publicError(err)
	}
	created, err := s.loadDelivery(ctx, newTaskNo)
	if err != nil || created == nil {
		return nil, publicError(errorOrCode(err, CodeUnavailable))
	}
	item := projectDelivery(*created)
	return &item, nil
}

func (s *serviceImpl) createManualResend(
	ctx context.Context,
	input ManualResendInput,
	task *deliveryRow,
	actor int,
	newTaskNo string,
) error {
	return s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var lockedChain chainRow
		if loadErr := s.database.Model(deliveryChainTable).Ctx(ctx).
			Where(tenantIDColumn, notificationTenantID).Where("chain_no", task.ChainNo).
			LockUpdate().Scan(&lockedChain); loadErr != nil {
			return loadErr
		}
		var lockedTask deliveryRow
		if loadErr := s.database.Model(deliveryTable).Ctx(ctx).
			Where(tenantIDColumn, notificationTenantID).Where("task_no", input.TaskNo).
			LockUpdate().Scan(&lockedTask); loadErr != nil {
			return loadErr
		}
		if lockedChain.ChainNo == "" || lockedTask.TaskNo == "" || lockedChain.TailTaskNo != input.TaskNo ||
			lockedTask.Version != input.ExpectedVersion || lockedTask.StatusCode != string(DeliveryFailed) ||
			!explicitNotReceived(lockedTask.ResultClass) || lockedChain.ManualResendCount >= maximumManualResends {
			return bizerr.NewCode(CodeConflict)
		}
		now, timeErr := s.databaseTime(ctx)
		if timeErr != nil {
			return timeErr
		}
		if lockedChain.LastManualResendAt != nil && now.Before(lockedChain.LastManualResendAt.UTC().Add(manualResendCooldown)) {
			return bizerr.NewCode(CodeConflict)
		}
		channel, channelErr := s.loadDeliveryChannel(ctx, lockedTask.ChannelCode)
		if channelErr != nil || !channelAvailableForManualResend(lockedTask.ChannelCode, channel) {
			return bizerr.NewCode(CodeConflict)
		}
		if _, insertErr := s.database.Model(deliveryTable).Ctx(ctx).Data(modeldo.DeliveryTask{
			TenantId: notificationTenantID, TaskNo: newTaskNo,
			ChainNo: lockedTask.ChainNo, PreviousTaskNo: lockedTask.TaskNo,
			EventCode: lockedTask.EventCode, TopicCode: lockedTask.TopicCode,
			ChannelCode: lockedTask.ChannelCode, TemplateNo: lockedTask.TemplateNo,
			TemplateVersion: lockedTask.TemplateVersion, LocaleCode: lockedTask.LocaleCode,
			OwnerUserId: lockedTask.OwnerUserID, CustomerNo: lockedTask.CustomerNo,
			ContactNo: lockedTask.ContactNo, RecipientMask: lockedTask.RecipientMask,
			BusinessObjectType: lockedTask.BusinessObjectType, BusinessObjectNo: lockedTask.BusinessObjectNo,
			BusinessObjectVersion: lockedTask.BusinessObjectVersion,
			MarketingSnapshotNo:   lockedTask.MarketingSnapshotNo,
			SubjectContent:        lockedTask.SubjectContent, BodyContent: lockedTask.BodyContent,
			ProviderVariables: lockedTask.ProviderVariables,
			ContentDigest:     lockedTask.ContentDigest, StatusCode: string(DeliveryQueued), ResultClass: "none",
		}).Insert(); insertErr != nil {
			return insertErr
		}
		updated, updateErr := s.database.Model(deliveryChainTable).Ctx(ctx).
			Where(tenantIDColumn, notificationTenantID).Where("chain_no", lockedChain.ChainNo).
			Where("tail_task_no", lockedTask.TaskNo).
			Where("version", lockedChain.Version).WhereLT("manual_resend_count", maximumManualResends).
			Data(modeldo.DeliveryChain{TailTaskNo: newTaskNo, ManualResendCount: gdb.Raw("manual_resend_count + 1"),
				LastManualResendAt: gdb.Raw("CURRENT_TIMESTAMP"), Version: gdb.Raw("version + 1")}).Update()
		if updateErr != nil || requireOne(updated) != nil {
			return bizerr.NewCode(CodeConflict)
		}
		if _, insertErr := s.database.Model("plugin_idc_notify_manual_resend").Ctx(ctx).Data(modeldo.ManualResend{
			TenantId: notificationTenantID, OperationNo: input.OperationNo,
			ChainNo:      lockedChain.ChainNo,
			SourceTaskNo: lockedTask.TaskNo, CreatedTaskNo: newTaskNo,
			ActorUserId: actor, ReasonText: strings.TrimSpace(input.Reason),
			SourceTaskVersion: input.ExpectedVersion,
		}).Insert(); insertErr != nil {
			return insertErr
		}
		if err := s.appendDeliveryAudit(ctx, lockedTask.TaskNo, "manual_resend_source", "staff", actor,
			"manual_resend", lockedTask.StatusCode, lockedTask.StatusCode, lockedTask.Version); err != nil {
			return err
		}
		return s.appendDeliveryAudit(ctx, newTaskNo, "manual_resend_created", "staff", actor,
			"manual_resend", "", string(DeliveryQueued), 1)
	})
}

func (s *serviceImpl) ManualCloseUnknown(ctx context.Context, input ManualClosureInput) (*Delivery, error) {
	if !stableCodePattern.MatchString(input.TaskNo) || input.ExpectedVersion <= 0 ||
		!validateIdempotencyKey(input.OperationNo) || !input.Confirmed ||
		strings.TrimSpace(input.Reason) == "" || len(input.Reason) > MaximumReasonBytes {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	actor, err := s.authorize(ctx, "notification-delivery-"+input.TaskNo)
	if err != nil {
		return nil, err
	}
	if err = s.requireRecentLogin(ctx, actor); err != nil {
		return nil, err
	}
	task, err := s.loadDelivery(ctx, input.TaskNo)
	if err != nil || task == nil {
		return nil, errorOrCode(err, CodeNotAccessible)
	}
	if err = s.authorizeBusinessObject(ctx, task); err != nil {
		return nil, err
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		result, updateErr := s.database.Model(deliveryTable).Ctx(ctx).
			Where(tenantIDColumn, notificationTenantID).Where("task_no", input.TaskNo).
			Where("version", input.ExpectedVersion).
			Where("status_code", string(DeliveryManualReview)).
			Data(modeldo.DeliveryTask{StatusCode: string(DeliveryManualUnknownClosed),
				ResultClass: "manually_closed_unknown", CompletedAt: gdb.Raw("CURRENT_TIMESTAMP"),
				Version: gdb.Raw("version + 1")}).Update()
		if updateErr != nil || requireOne(result) != nil {
			return bizerr.NewCode(CodeConflict)
		}
		if _, insertErr := s.database.Model("plugin_idc_notify_manual_closure").Ctx(ctx).Data(modeldo.ManualClosure{
			TenantId: notificationTenantID, ClosureNo: input.OperationNo,
			TaskNo: input.TaskNo, ActorUserId: actor,
			ReasonText: strings.TrimSpace(input.Reason), ExpectedTaskVersion: input.ExpectedVersion,
		}).Insert(); insertErr != nil {
			return insertErr
		}
		return s.appendDeliveryAudit(ctx, input.TaskNo, "manual_unknown_close", "staff", actor,
			"manual_unknown_close", string(DeliveryManualReview), string(DeliveryManualUnknownClosed), input.ExpectedVersion+1)
	})
	if err != nil {
		return nil, publicError(err)
	}
	closed, err := s.loadDelivery(ctx, input.TaskNo)
	if err != nil || closed == nil {
		return nil, publicError(errorOrCode(err, CodeUnavailable))
	}
	item := projectDelivery(*closed)
	return &item, nil
}

func (s *serviceImpl) authorizeBusinessObject(ctx context.Context, task *deliveryRow) error {
	if task == nil || task.BusinessObjectNo == "" {
		return bizerr.NewCode(CodeNotAccessible)
	}
	projection, err := s.sources.BusinessObjects.AuthorizeCurrent(ctx, notificationcap.BusinessObjectRequest{
		Type: notificationcap.BusinessObjectType(task.BusinessObjectType), ObjectNo: task.BusinessObjectNo,
	})
	if err != nil || projection.ObjectNo != task.BusinessObjectNo ||
		string(projection.Type) != task.BusinessObjectType || projection.CurrentVersion < 1 {
		return errorOrCode(err, CodeNotAccessible)
	}
	return nil
}

func (s *serviceImpl) recipientCurrentlyBlocked(ctx context.Context, task *deliveryRow) (blocked bool, unsafe bool) {
	if task == nil || !customerAddressChannel(task.ChannelCode) {
		return false, false
	}
	request := notificationcap.ContactRequest{CustomerNo: task.CustomerNo, ContactNo: task.ContactNo,
		Channel: notificationcap.Channel(task.ChannelCode), Topic: notificationcap.Topic(task.TopicCode)}
	called := false
	err := s.sources.Contacts.ResolveBatch(ctx, []notificationcap.ContactRequest{request}, func(_ int, projection notificationcap.ContactProjection, destination []byte) error {
		called = true
		if projection.CustomerNo != task.CustomerNo || projection.ContactNo != task.ContactNo || len(destination) == 0 {
			unsafe = true
			return nil
		}
		blocked = projection.PreferencePresent && !projection.PreferenceEnabled
		return nil
	})
	if err != nil || !called {
		return false, true
	}
	if task.TopicCode == string(notificationcap.TopicMarketing) && !blocked {
		_, err = s.evaluateMarketing(ctx, EnqueueInput{EventCode: task.EventCode, ChannelCode: task.ChannelCode,
			CustomerNo: task.CustomerNo, ContactNo: task.ContactNo, BusinessObjectType: task.BusinessObjectType,
			BusinessObjectNo: task.BusinessObjectNo, MarketingSnapshotNo: task.MarketingSnapshotNo}, task.ContentDigest)
		unsafe = err != nil
	}
	return blocked, unsafe
}

func (s *serviceImpl) loadManualResendReplay(ctx context.Context, operationNo, sourceTaskNo string) (string, error) {
	var rows []struct {
		SourceTaskNo  string `orm:"source_task_no"`
		CreatedTaskNo string `orm:"created_task_no"`
	}
	err := s.database.Model("plugin_idc_notify_manual_resend").Ctx(ctx).
		Where(tenantIDColumn, notificationTenantID).Where("operation_no", operationNo).Limit(1).Scan(&rows)
	if err != nil {
		return "", err
	}
	if len(rows) == 0 || rows[0].CreatedTaskNo == "" {
		return "", nil
	}
	row := rows[0]
	if row.SourceTaskNo != sourceTaskNo {
		return "", bizerr.NewCode(CodeConflict)
	}
	return row.CreatedTaskNo, nil
}

func (s *serviceImpl) databaseTime(ctx context.Context) (time.Time, error) {
	value, err := s.database.GetValue(ctx, "SELECT CURRENT_TIMESTAMP")
	if err != nil {
		return time.Time{}, err
	}
	now := value.Time()
	if now.IsZero() {
		return time.Time{}, bizerr.NewCode(CodeUnavailable)
	}
	return now.UTC(), nil
}

func explicitNotReceived(result string) bool {
	return result == "definitive_not_received_temporary" || result == "definitive_not_received_permanent"
}

func channelAvailableForManualResend(channelCode string, channel *deliveryChannelRow) bool {
	if channel == nil || ChannelStatus(channel.StatusCode) != ChannelEnabled || channel.BalanceState != "sufficient" {
		return false
	}
	return channelCode == string(notificationcap.ChannelInApp) ||
		(channel.ProviderCode != nil && channel.CredentialSecretRef != nil)
}
