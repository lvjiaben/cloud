package customer360summary

import (
	"context"
	"testing"

	"github.com/gogf/gf/v2/container/gvar"
	"github.com/gogf/gf/v2/database/gdb"
)

type countingQuery struct{ calls int }

func (query *countingQuery) GetAll(
	_ context.Context,
	_ string,
	_ ...any,
) (gdb.Result, error) {
	query.calls++
	return gdb.Result{
		gdb.Record{"customer_no": gvar.New("10000"), "total": gvar.New(2)},
		gdb.Record{"customer_no": gvar.New("10001"), "total": gvar.New(0)},
	}, nil
}

func TestBatchCountUsesOneBoundedSetQuery(t *testing.T) {
	query := &countingQuery{}
	result, err := batchCount(context.Background(), query, []string{"10000", "10001"})
	if err != nil || query.calls != 1 || len(result) != 2 || result[0].Total != 2 {
		t.Fatalf("result=%+v calls=%d err=%v", result, query.calls, err)
	}
	if _, err = batchCount(context.Background(), query, []string{"10000", "10000"}); err == nil {
		t.Fatal("duplicate customer batch was accepted")
	}
	if query.calls != 1 {
		t.Fatalf("invalid batch reached database: calls=%d", query.calls)
	}
}
