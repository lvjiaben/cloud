// Customer audit projection tests verify bounded owner queries and secret omission.

package admin

import (
	"context"
	"encoding/json"
	"errors"
	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/audit"
	"lina-plugin-idc-core/backend/internal/service/customermanagement"
	"strings"
	"testing"
	"time"
)

type auditCustomerOverviewStub struct {
	customermanagement.Service
	overview *customermanagement.Overview
	getNo    string
}

func (s *auditCustomerOverviewStub) GetOverview(_ context.Context, no string) (*customermanagement.Overview, error) {
	s.getNo = no
	return s.overview, nil
}

type customerAuditReaderStub struct{ input audit.SearchInput }

func (s *customerAuditReaderStub) Search(_ context.Context, input audit.SearchInput) (*audit.Page, error) {
	s.input = input
	return &audit.Page{Events: []audit.Event{{AuditNo: "AUD_0123456789abcdef0123456789abcdef", OccurredAt: time.UnixMilli(1780000000000), OperationCode: "customer.password.reset", Result: audit.ResultSucceeded, BeforeSnapshot: json.RawMessage(`{"private":"secret-value"}`)}}}, nil
}

func TestCustomerAuditListUsesOwnerPageLimitAndDropsSnapshots(t *testing.T) {
	management := &auditCustomerOverviewStub{overview: &customermanagement.Overview{}}
	reader := &customerAuditReaderStub{}
	controller := &ControllerV1{customerManagement: management, customerAudit: reader}
	req := &v1.CustomerAuditListReq{CustomerNo: "CUS_0123456789abcdef0123456789abcdef"}
	result, err := controller.CustomerAuditList(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if management.getNo != req.CustomerNo || reader.input.CustomerNo != req.CustomerNo || reader.input.Limit != 0 {
		t.Fatalf("scope or configured limit lost: %+v", reader.input)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "secret-value") || len(result.Items) != 1 || result.Items[0].OccurredAt != 1780000000000 {
		t.Fatal("invalid display-safe projection")
	}
	if err = BindCustomerAudit(nil, reader); err == nil {
		t.Fatal("nil controller accepted")
	}
}

// A rejected customer scope stops before the audit owner is queried.
type deniedCustomerOverview struct{ auditCustomerOverviewStub }

func (*deniedCustomerOverview) GetOverview(context.Context, string) (*customermanagement.Overview, error) {
	return nil, errors.New("out of scope")
}
func TestCustomerAuditListRejectsInvisibleCustomerBeforeQuery(t *testing.T) {
	reader := &customerAuditReaderStub{}
	controller := &ControllerV1{customerManagement: &deniedCustomerOverview{}, customerAudit: reader}
	if _, err := controller.CustomerAuditList(context.Background(), &v1.CustomerAuditListReq{CustomerNo: "CUS_0123456789abcdef0123456789abcdef"}); err == nil {
		t.Fatal("invisible customer accepted")
	}
	if reader.input.CustomerNo != "" {
		t.Fatal("audit reader queried before scope check")
	}
}
