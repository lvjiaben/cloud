package customer360summary

import (
	"context"
	"errors"
	"testing"

	"lina-plugin-idc-core/backend/cap/customer360cap"
)

type contributorStub struct {
	codes  []customer360cap.Code
	counts []customer360cap.Count
	err    error
}

func (s contributorStub) Codes() []customer360cap.Code { return s.codes }
func (s contributorStub) BatchCount(context.Context, []string) ([]customer360cap.Count, error) {
	return s.counts, s.err
}

func completeStubs(customerNo string) []customer360cap.Contributor {
	result := make([]customer360cap.Contributor, 0, len(requiredCodes))
	for index, code := range requiredCodes {
		result = append(result, contributorStub{codes: []customer360cap.Code{code}, counts: []customer360cap.Count{{
			CustomerNo: customerNo, Code: code, Total: int64(index + 1),
		}}})
	}
	return result
}

func TestReaderRequiresCompleteUniqueAnswers(t *testing.T) {
	contributors := completeStubs("CUS_test")
	service, err := New(contributors)
	if err != nil {
		t.Fatal(err)
	}
	summary, err := service.Get(context.Background(), "CUS_test")
	if err != nil || summary.Orders != 1 || summary.Audits != 8 {
		t.Fatalf("summary=%+v err=%v", summary, err)
	}
	broken := completeStubs("CUS_test")
	broken[0] = contributorStub{codes: []customer360cap.Code{customer360cap.CodeOrders}}
	service, err = New(broken)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Get(context.Background(), "CUS_test"); err == nil {
		t.Fatal("missing answer should fail closed")
	}
}

func TestReaderPropagatesContributorFailure(t *testing.T) {
	contributors := completeStubs("CUS_test")
	contributors[0] = contributorStub{codes: []customer360cap.Code{customer360cap.CodeOrders}, err: errors.New("down")}
	service, err := New(contributors)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Get(context.Background(), "CUS_test"); err == nil {
		t.Fatal("contributor failure should fail closed")
	}
}
