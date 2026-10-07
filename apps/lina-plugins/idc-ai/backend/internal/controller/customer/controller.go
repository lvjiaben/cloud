package customer

import (
	"context"
	"strconv"

	"github.com/gogf/gf/v2/errors/gerror"
	v1 "lina-plugin-idc-ai/backend/api/customer/v1"
	"lina-plugin-idc-ai/backend/internal/service/copilot"
	"lina-plugin-idc-core/backend/cap/customerauthcap"
)

type ControllerV1 struct {
	service   *copilot.Service
	customers customerauthcap.Reader
}

func NewV1(s *copilot.Service, customers customerauthcap.Reader) *ControllerV1 {
	return &ControllerV1{service: s, customers: customers}
}
func (c *ControllerV1) AssistantStatus(ctx context.Context, _ *v1.AssistantStatusReq) (*v1.AssistantStatusRes, error) {
	if _, err := c.currentCustomer(ctx); err != nil {
		return nil, err
	}
	x, e := c.service.GetConfig(ctx)
	if e != nil {
		return nil, e
	}
	return &v1.AssistantStatusRes{Enabled: x.Enabled}, nil
}
func (c *ControllerV1) AssistantChat(ctx context.Context, r *v1.AssistantChatReq) (*v1.AssistantChatRes, error) {
	customer, err := c.currentCustomer(ctx)
	if err != nil {
		return nil, err
	}
	actorID := customer.PrincipalReference
	if actorID == "" {
		actorID = "user:" + strconv.Itoa(customer.ActorUserID)
	}
	a, evidence, d, u, safety, model, conversationID, e := c.service.Chat(ctx, r.Prompt, "customer", actorID)
	_ = evidence
	_ = safety
	_ = model
	_ = conversationID
	if e != nil {
		return nil, e
	}
	nav := "/account"
	if len(r.Prompt) > 0 {
		nav = "/account/support"
	}
	return &v1.AssistantChatRes{Answer: a, Navigation: nav, DataAsOf: d, Uncertainty: u}, nil
}

func (c *ControllerV1) currentCustomer(ctx context.Context) (*customerauthcap.Subject, error) {
	if c == nil || c.customers == nil {
		return nil, gerror.New("customer assistant identity source is unavailable")
	}
	customer, err := c.customers.CurrentCustomer(ctx)
	if err != nil {
		return nil, err
	}
	if customer == nil || customer.CustomerNo == "" || customer.ActorUserID <= 0 {
		return nil, gerror.New("customer assistant requires an active customer login")
	}
	return customer, nil
}
