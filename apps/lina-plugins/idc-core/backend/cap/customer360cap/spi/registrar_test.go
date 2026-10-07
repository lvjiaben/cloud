package spi

import (
	"context"
	"testing"

	"lina-plugin-idc-core/backend/cap/customer360cap"
)

type testContributor struct{ codes []customer360cap.Code }

func (c testContributor) Codes() []customer360cap.Code { return c.codes }
func (testContributor) BatchCount(context.Context, []string) ([]customer360cap.Count, error) {
	return nil, nil
}

func TestRegistrarRejectsDuplicateAndIncompleteCoverage(t *testing.T) {
	r := &registrar{}
	core := []customer360cap.Code{
		customer360cap.CodeOrders, customer360cap.CodeBills, customer360cap.CodeTransactions,
		customer360cap.CodeServers, customer360cap.CodeTickets, customer360cap.CodeAudits,
	}
	if err := r.declare(testContributor{codes: core}); err != nil {
		t.Fatal(err)
	}
	if err := r.declare(testContributor{codes: []customer360cap.Code{customer360cap.CodeOrders}}); err == nil {
		t.Fatal("duplicate ownership should fail")
	}
	if _, err := r.freeze(); err == nil {
		t.Fatal("incomplete coverage should fail closed")
	}
}

func TestRegistrarFreezesCompleteCoverage(t *testing.T) {
	r := &registrar{}
	all := []customer360cap.Code{
		customer360cap.CodeOrders, customer360cap.CodeBills, customer360cap.CodeTransactions,
		customer360cap.CodeServers, customer360cap.CodeContracts, customer360cap.CodeTaxInvoices,
		customer360cap.CodeTickets, customer360cap.CodeNotifications, customer360cap.CodeAudits,
	}
	for _, code := range all {
		if err := r.declare(testContributor{codes: []customer360cap.Code{code}}); err != nil {
			t.Fatal(err)
		}
	}
	if snapshot, err := r.freeze(); err != nil || len(snapshot) != len(all) {
		t.Fatalf("freeze result=%d err=%v", len(snapshot), err)
	}
	if err := r.declare(testContributor{codes: []customer360cap.Code{customer360cap.CodeOrders}}); err == nil {
		t.Fatal("declaration after freeze should fail")
	}
}
