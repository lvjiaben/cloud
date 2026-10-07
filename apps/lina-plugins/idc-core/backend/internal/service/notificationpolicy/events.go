package notificationpolicy

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/notificationcap"
	"lina-plugin-idc-core/backend/internal/service/outbox"
)

const notificationEventMaximumVariables = 30

type notificationEventEnvelope struct {
	SourcePlugin string                       `json:"sourcePlugin"`
	Request      notificationcap.EventRequest `json:"request"`
}

type eventSink struct {
	sourcePlugin string
	outbox       outbox.Enqueuer
}

type eventFeed struct{ outbox outbox.Service }

var (
	_ notificationcap.EventSink = (*eventSink)(nil)
	_ notificationcap.EventFeed = (*eventFeed)(nil)
)

func NewEventSink(sourcePlugin string, writer outbox.Enqueuer) (notificationcap.EventSink, error) {
	if !allowedNotificationEventSource(sourcePlugin) || writer == nil {
		return nil, gerror.New("notification event source is not allowed")
	}
	return &eventSink{sourcePlugin: sourcePlugin, outbox: writer}, nil
}

func NewEventFeed(dispatcher outbox.Service) (notificationcap.EventFeed, error) {
	if dispatcher == nil {
		return nil, gerror.New("notification event feed requires outbox dispatcher")
	}
	return &eventFeed{outbox: dispatcher}, nil
}

func (s *eventSink) Enqueue(ctx context.Context, request notificationcap.EventRequest) (notificationcap.EventReceipt, error) {
	if err := validateNotificationEventRequest(request); err != nil {
		return notificationcap.EventReceipt{}, err
	}
	payload, err := json.Marshal(notificationEventEnvelope{SourcePlugin: s.sourcePlugin, Request: request})
	if err != nil {
		return notificationcap.EventReceipt{}, gerror.Wrap(err, "encode notification event")
	}
	event, err := s.outbox.Enqueue(ctx, outbox.EnqueueInput{
		EventType:        notificationcap.NotificationRequestedEventType,
		EventVersion:     1,
		Topic:            "idc.notification.requested",
		AggregateType:    string(request.BusinessObjectType),
		AggregateNo:      request.BusinessObjectNo,
		CorrelationNo:    request.CorrelationNo,
		DeduplicationKey: s.sourcePlugin + ":" + request.IdempotencyKey,
		Payload:          payload,
		Metadata:         json.RawMessage(`{}`),
	})
	if err != nil {
		return notificationcap.EventReceipt{}, err
	}
	return eventReceipt(event, s.sourcePlugin), nil
}

func (f *eventFeed) Claim(ctx context.Context, input notificationcap.EventClaimInput) ([]notificationcap.ClaimedEvent, error) {
	claimed, err := f.outbox.Claim(ctx, outbox.ClaimInput{
		WorkerID: input.WorkerID,
		Limit:    input.Limit,
		EventTypes: []string{
			notificationcap.NotificationRequestedEventType,
		},
	})
	if err != nil {
		return nil, err
	}
	result := make([]notificationcap.ClaimedEvent, 0, len(claimed))
	for _, item := range claimed {
		envelope, decodeErr := decodeNotificationEvent(item.Payload)
		if decodeErr != nil {
			return nil, decodeErr
		}
		result = append(result, notificationcap.ClaimedEvent{
			EventReceipt: eventReceipt(&item.Event, envelope.SourcePlugin),
			Request:      envelope.Request,
			Lease: notificationcap.EventLease{
				EventNo: item.EventNo, LeaseToken: item.LeaseToken, AttemptNo: item.AttemptNo,
			},
			LeaseExpiresAt: item.LeaseExpiresAt,
		})
	}
	return result, nil
}

func (f *eventFeed) Ack(ctx context.Context, lease notificationcap.EventLease) (notificationcap.EventReceipt, error) {
	event, err := f.outbox.MarkPublished(ctx, outbox.LeaseRef{EventNo: lease.EventNo, LeaseToken: lease.LeaseToken, AttemptNo: lease.AttemptNo})
	if err != nil {
		return notificationcap.EventReceipt{}, err
	}
	envelope, err := decodeNotificationEvent(event.Payload)
	if err != nil {
		return notificationcap.EventReceipt{}, err
	}
	return eventReceipt(event, envelope.SourcePlugin), nil
}

func (f *eventFeed) Release(ctx context.Context, input notificationcap.EventReleaseInput) (notificationcap.EventReceipt, error) {
	event, err := f.outbox.RecordFailure(ctx, outbox.FailureInput{
		Lease: outbox.LeaseRef{
			EventNo: input.Lease.EventNo, LeaseToken: input.Lease.LeaseToken, AttemptNo: input.Lease.AttemptNo,
		},
		Retryable: input.Retryable, ErrorCode: input.ErrorCode, ErrorSummary: input.ErrorSummary,
	})
	if err != nil {
		return notificationcap.EventReceipt{}, err
	}
	envelope, err := decodeNotificationEvent(event.Payload)
	if err != nil {
		return notificationcap.EventReceipt{}, err
	}
	return eventReceipt(event, envelope.SourcePlugin), nil
}

func eventReceipt(event *outbox.Event, sourcePlugin string) notificationcap.EventReceipt {
	if event == nil {
		return notificationcap.EventReceipt{}
	}
	return notificationcap.EventReceipt{
		EventNo: event.EventNo, SourcePlugin: sourcePlugin, Status: string(event.Status),
		OccurredAt: event.OccurredAt, AvailableAt: event.AvailableAt, CorrelationNo: event.CorrelationNo,
	}
}

func decodeNotificationEvent(payload json.RawMessage) (notificationEventEnvelope, error) {
	var envelope notificationEventEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return notificationEventEnvelope{}, gerror.Wrap(err, "decode notification event")
	}
	if !allowedNotificationEventSource(envelope.SourcePlugin) {
		return notificationEventEnvelope{}, gerror.New("notification event source is invalid")
	}
	if err := validateNotificationEventRequest(envelope.Request); err != nil {
		return notificationEventEnvelope{}, err
	}
	return envelope, nil
}

func validateNotificationEventRequest(request notificationcap.EventRequest) error {
	request.EventCode = strings.TrimSpace(request.EventCode)
	request.BusinessObjectNo = strings.TrimSpace(request.BusinessObjectNo)
	request.CorrelationNo = strings.TrimSpace(request.CorrelationNo)
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
	if !notificationIdentifierPattern.MatchString(request.EventCode) ||
		!validBusinessObjectType(request.BusinessObjectType) || !notificationIdentifierPattern.MatchString(request.BusinessObjectNo) ||
		request.BusinessObjectVersion < 0 || len(request.CorrelationNo) > 128 ||
		len(request.IdempotencyKey) < 8 || len(request.IdempotencyKey) > 128 || len(request.Variables) > notificationEventMaximumVariables {
		return gerror.New("notification event request is invalid")
	}
	if !validNotificationRecipient(request.Recipient) {
		return gerror.New("notification recipient is invalid")
	}
	if request.BusinessObjectType == notificationcap.BusinessObjectMarketingCampaign && request.MarketingSnapshotNo == "" {
		return gerror.New("marketing notification snapshot is missing")
	}
	total := 0
	for key, value := range request.Variables {
		if !notificationIdentifierPattern.MatchString(key) || len(value) > 4096 {
			return gerror.New("notification event variable is invalid")
		}
		total += len(key) + len(value)
	}
	if total > 65536 {
		return gerror.New("notification event variables are too large")
	}
	return nil
}

func validNotificationRecipient(recipient notificationcap.Recipient) bool {
	validCustomer := notificationIdentifierPattern.MatchString(strings.TrimSpace(recipient.CustomerNo))
	validContact := notificationIdentifierPattern.MatchString(strings.TrimSpace(recipient.ContactNo))
	validDestination := notificationIdentifierPattern.MatchString(strings.TrimSpace(recipient.DestinationNo))
	switch recipient.Kind {
	case notificationcap.RecipientCustomerContact:
		return validCustomer && validContact && recipient.StaffUserID == 0 && recipient.DestinationNo == ""
	case notificationcap.RecipientStaffUser:
		return recipient.StaffUserID > 0 && recipient.CustomerNo == "" && recipient.ContactNo == "" && recipient.DestinationNo == ""
	case notificationcap.RecipientVerifiedDestination:
		return validDestination && recipient.CustomerNo == "" && recipient.ContactNo == "" && recipient.StaffUserID == 0
	default:
		return false
	}
}

func validBusinessObjectType(value notificationcap.BusinessObjectType) bool {
	switch value {
	case notificationcap.BusinessObjectSupportTicket, notificationcap.BusinessObjectContentItem,
		notificationcap.BusinessObjectMarketingCampaign, notificationcap.BusinessObjectAccount,
		notificationcap.BusinessObjectService, notificationcap.BusinessObjectBilling:
		return true
	default:
		return false
	}
}

func allowedNotificationEventSource(pluginID string) bool {
	switch pluginID {
	case "idc-core", "idc-payment", "idc-resource", "idc-zjmfcloud",
		"idc-compliance", "idc-channel", "idc-ai", "idc-portal",
		"idc-zjmfmigrate":
		return true
	default:
		return false
	}
}
