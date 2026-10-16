// This file verifies entered tag bounds before persistence is attempted.
package customermanagement

import (
	"context"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
)

func TestNamedTagsValidation(t *testing.T) {
	input := TagBatchInput{Action: TagBatchAssign, CustomerNos: []string{"CUS_0123456789abcdef0123456789abcdef"}, TagNames: []string{" 新标签 "}, IdempotencyKey: "named-tag-test-0001"}
	normalized, err := normalizeTagBatch(input)
	if err != nil || len(normalized.tagNames) != 1 || normalized.tagNames[0] != "新标签" || normalized.reason == "" {
		t.Fatalf("normalized=%+v err=%v", normalized, err)
	}
	for _, names := range [][]string{{""}, {strings.Repeat("x", 129)}, {"a", " a "}, make([]string, 21)} {
		input.TagNames = names
		if _, err = normalizeTagBatch(input); err == nil {
			t.Fatalf("invalid names accepted: %v", names)
		}
	}
	input.TagNames = []string{"new"}
	input.Action = TagBatchRevoke
	if _, err = normalizeTagBatch(input); err == nil {
		t.Fatal("removal created a tag")
	}
}

// Concurrent transactions must resolve one shared definition without a process lock.
func TestNamedTagsConcurrentPostgres(t *testing.T) {
	ctx := context.Background()
	configureManagementIntegrationDB(t, ctx)
	var (
		start   = make(chan struct{})
		results = make(chan error, 2)
	)
	for range 2 {
		go func() {
			<-start
			results <- dao.CustomerTag.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
				_, err := ensureNamedTags(txCtx, []string{"并发输入标签"}, 71)
				return err
			})
		}()
	}
	close(start)
	for range 2 {
		if err := <-results; err != nil {
			t.Errorf("concurrent tag transaction: %v", err)
		}
	}
	count, err := dao.CustomerTag.Ctx(ctx).Where(do.CustomerTag{TenantId: 0, NameZhCn: "并发输入标签"}).Count()
	if err != nil || count != 1 {
		t.Fatalf("shared definitions=%d err=%v", count, err)
	}
}
