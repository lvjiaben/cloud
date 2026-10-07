package customer

import (
	"context"
	"errors"
	"testing"

	"lina-plugin-idc-ai/backend/api/customer/v1"
	"lina-plugin-idc-ai/backend/internal/service/copilot"
	"lina-plugin-idc-core/backend/cap/customerauthcap"
)

type currentCustomerStub struct {
	subject *customerauthcap.Subject
	err     error
}

func (stub currentCustomerStub) CurrentCustomer(context.Context) (*customerauthcap.Subject, error) {
	return stub.subject, stub.err
}

func TestAssistantStatusRequiresCurrentCustomer(t *testing.T) {
	cause := errors.New("not a customer")
	controller := NewV1(copilot.New(nil), currentCustomerStub{err: cause})
	if _, err := controller.AssistantStatus(context.Background(), &v1.AssistantStatusReq{}); !errors.Is(err, cause) {
		t.Fatalf("customer identity error was not preserved: %v", err)
	}
}

func TestAssistantChatUsesProvenCustomer(t *testing.T) {
	controller := NewV1(copilot.New(nil), currentCustomerStub{subject: &customerauthcap.Subject{
		ActorUserID:        42,
		CustomerNo:         "CUS_42",
		PrincipalReference: "usr:42",
	}})
	response, err := controller.AssistantChat(context.Background(), &v1.AssistantChatReq{
		Prompt: "解释我的服务状态",
	})
	if err != nil {
		t.Fatalf("assistant chat failed: %v", err)
	}
	if response.Navigation != "/account/support" || response.Answer == "" {
		t.Fatalf("unexpected assistant response: %+v", response)
	}
}
