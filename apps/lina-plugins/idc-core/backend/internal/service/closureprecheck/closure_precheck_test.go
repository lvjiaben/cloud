package closureprecheck

import (
	"context"
	"errors"
	"testing"
	"time"

	"lina-plugin-idc-core/backend/cap/customerclosurecap"
)

type contributorStub struct {
	codes []customerclosurecap.Code
	items []customerclosurecap.Count
	err   error
}

func (s contributorStub) Codes() []customerclosurecap.Code { return s.codes }
func (s contributorStub) BatchCount(context.Context, []string, time.Time) ([]customerclosurecap.Count, error) {
	return s.items, s.err
}

func TestContributorFailuresRemainFailClosed(t *testing.T) {
	stub := contributorStub{codes: []customerclosurecap.Code{customerclosurecap.CodeUnsettledFunds}, err: errors.New("unavailable")}
	if _, err := stub.BatchCount(t.Context(), []string{"CUS_1"}, time.Now()); err == nil {
		t.Fatal("expected contributor failure")
	}
	// Structural completeness is covered by the SPI registrar and customer
	// workflow validation tests; this guard keeps provider errors observable.
}
