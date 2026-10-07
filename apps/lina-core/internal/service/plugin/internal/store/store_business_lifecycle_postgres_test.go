package store

import (
	"context"
	"testing"

	"github.com/gogf/gf/v2/frame/g"
)

func TestBusinessLifecyclePostgreSQLSchema(t *testing.T) {
	ctx := context.Background()
	columns, err := g.DB().GetAll(ctx, `
SELECT column_name, data_type, is_nullable, COALESCE(column_default, '') AS column_default
FROM information_schema.columns
WHERE table_name = 'sys_plugin'
  AND column_name IN ('new_business_state', 'new_business_stopped_at')
ORDER BY column_name`)
	if err != nil {
		t.Fatalf("query plugin business lifecycle columns: %v", err)
	}
	if len(columns) != 2 {
		t.Fatalf("expected two plugin business lifecycle columns, got %#v", columns)
	}

	constraintCount, err := g.DB().GetCount(ctx, `
SELECT 1
FROM pg_constraint
WHERE conrelid = 'sys_plugin'::regclass
  AND conname = 'ck_sys_plugin_new_business_state'`)
	if err != nil || constraintCount != 1 {
		t.Fatalf("expected bounded new-business state constraint, count=%d err=%v", constraintCount, err)
	}

	permissionCount, err := g.DB().GetCount(ctx, `
SELECT 1
FROM sys_menu
WHERE menu_key IN (
  'extension:plugin:business-impact',
  'extension:plugin:stop-new-business',
  'extension:plugin:resume-new-business'
)`)
	if err != nil || permissionCount != 3 {
		t.Fatalf("expected three business lifecycle permissions, count=%d err=%v", permissionCount, err)
	}
}
