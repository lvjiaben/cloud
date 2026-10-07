package spi

import (
	"context"
	"testing"
	"time"

	"lina-plugin-idc-core/backend/cap/customerclosurecap"
)

type testContributor struct{ codes []customerclosurecap.Code }

func (c testContributor) Codes() []customerclosurecap.Code { return c.codes }
func (testContributor) BatchCount(context.Context, []string, time.Time) ([]customerclosurecap.Count, error) {
	return nil, nil
}

func TestRegistrarRequiresCompleteUniqueCoverage(t *testing.T) {
	r := &registrar{}
	coreCodes := []customerclosurecap.Code{
		customerclosurecap.CodeUnsettledFunds,
		customerclosurecap.CodeActiveServices,
		customerclosurecap.CodeUnsettledInvoices,
		customerclosurecap.CodeInProgressOperations,
	}
	complianceCodes := []customerclosurecap.Code{
		customerclosurecap.CodeActiveContracts,
		customerclosurecap.CodeLegalRetention,
	}
	if err := r.declare(testContributor{codes: coreCodes}); err != nil {
		t.Fatalf("declare core contributor: %v", err)
	}
	if err := r.declare(testContributor{codes: complianceCodes}); err != nil {
		t.Fatalf("declare compliance contributor: %v", err)
	}
	if _, err := r.freeze(); err != nil {
		t.Fatalf("freeze complete coverage: %v", err)
	}
	if err := r.declare(testContributor{codes: complianceCodes}); err == nil {
		t.Fatal("expected declaration after freeze to fail")
	}
}

func TestRegistrarRejectsMissingAndDuplicateCoverage(t *testing.T) {
	r := &registrar{}
	if err := r.declare(testContributor{codes: []customerclosurecap.Code{customerclosurecap.CodeUnsettledFunds}}); err != nil {
		t.Fatalf("declare first contributor: %v", err)
	}
	if err := r.declare(testContributor{codes: []customerclosurecap.Code{customerclosurecap.CodeUnsettledFunds}}); err == nil {
		t.Fatal("expected duplicate owner to fail")
	}
	if _, err := r.freeze(); err == nil {
		t.Fatal("expected incomplete coverage to fail")
	}
}
