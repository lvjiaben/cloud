package public

import (
	"context"

	"lina-plugin-idc-core/backend/cap/supportcap"
	"lina-plugin-idc-notify/backend/api/public/v1"
	"lina-plugin-idc-notify/backend/internal/service/notification"
)

func (c *ControllerV1) TicketCreate(ctx context.Context, req *v1.TicketCreateReq) (res *v1.TicketCreateRes, err error) {
	browser, destination, code := []byte(req.BrowserNonce), []byte(req.Destination), []byte(req.Code)
	req.BrowserNonce, req.Destination, req.Code = "", "", ""
	item, err := c.notifications.CreateAnonymousPresalesTicket(ctx, notification.PresalesTicketInput{
		BrowserNonce: browser, Destination: destination, DestinationKind: req.DestinationKind, Locale: req.Locale,
		OperationNo: req.OperationNo, VerificationNo: req.VerificationNo, Code: code,
		CategoryNo: req.CategoryNo, Subject: req.Subject, Body: req.Body,
		CustomFields: supportCustomFieldValues(req.CustomFields),
	})
	if err != nil {
		return nil, err
	}
	return (*v1.TicketCreateRes)(accessProjection(item)), nil
}

func supportCustomFieldValues(items []v1.TicketCustomFieldSubmission) []supportcap.CustomFieldValue {
	result := make([]supportcap.CustomFieldValue, 0, len(items))
	for _, item := range items {
		result = append(result, supportcap.CustomFieldValue{
			FieldNo: item.FieldNo, TextValue: item.TextValue, NumberValue: item.NumberValue,
			DateValue: item.DateValue, BooleanValue: item.BooleanValue,
			OptionNos: append([]string(nil), item.OptionNos...),
		})
	}
	return result
}
