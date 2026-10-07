package customer

import (
	"context"
	v1 "lina-plugin-idc-ai/backend/api/customer/v1"
)

type ICustomerV1 interface {
	AssistantStatus(context.Context, *v1.AssistantStatusReq) (*v1.AssistantStatusRes, error)
	AssistantChat(context.Context, *v1.AssistantChatReq) (*v1.AssistantChatRes, error)
}
