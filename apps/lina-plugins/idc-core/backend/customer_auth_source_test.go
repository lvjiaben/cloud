package backend

import (
	"context"
	"errors"
	"testing"

	"lina-plugin-idc-core/backend/cap/customerauthcap"
	"lina-plugin-idc-core/backend/cap/identitycap"
)

type customerAuthTestReader struct {
	subject *identitycap.CustomerSubject
	err     error
}

func (reader customerAuthTestReader) CurrentCustomer(context.Context) (*identitycap.CustomerSubject, error) {
	return reader.subject, reader.err
}

func TestCurrentCustomerAdapterReturnsOnlyBoundedSubject(t *testing.T) {
	adapter := currentCustomerReaderAdapter{reader: customerAuthTestReader{subject: &identitycap.CustomerSubject{
		CustomerNo: "customer-1", CustomerTypeCode: "enterprise",
		PrincipalReference: "usr:42", ActorUserID: 42,
	}}}
	subject, err := adapter.CurrentCustomer(context.Background())
	if err != nil || subject == nil || subject.CustomerNo != "customer-1" ||
		subject.CustomerTypeCode != "enterprise" || subject.PrincipalReference != "usr:42" ||
		subject.ActorUserID != 42 {
		t.Fatalf("bounded customer subject=%+v err=%v", subject, err)
	}
}

func TestCurrentCustomerAdapterFailsClosed(t *testing.T) {
	if _, err := (currentCustomerReaderAdapter{}).CurrentCustomer(context.Background()); err == nil {
		t.Fatal("missing current customer reader was accepted")
	}
	cause := errors.New("identity unavailable")
	adapter := currentCustomerReaderAdapter{reader: customerAuthTestReader{err: cause}}
	if _, err := adapter.CurrentCustomer(context.Background()); !errors.Is(err, cause) {
		t.Fatalf("identity error was not preserved: %v", err)
	}
	if _, err := newSourceCurrentCustomerReader(context.Background(), nil,
		customerauthcap.AllowedCallerPluginID); err == nil {
		t.Fatal("missing host services were accepted")
	}
}

func TestCurrentCustomerSourceAllowsOnlyDeclaredConsumers(t *testing.T) {
	for _, pluginID := range []string{
		customerauthcap.AllowedCallerPluginID,
		customerauthcap.AllowedAICallerPluginID,
	} {
		if !customerauthcap.IsAllowedCallerPlugin(pluginID) {
			t.Fatalf("declared current customer consumer %q was rejected", pluginID)
		}
	}
	if customerauthcap.IsAllowedCallerPlugin("unknown-plugin") {
		t.Fatal("undeclared current customer consumer was accepted")
	}
}
