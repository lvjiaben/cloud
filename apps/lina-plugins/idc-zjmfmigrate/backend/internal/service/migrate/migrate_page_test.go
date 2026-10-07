package migrate

import (
	"context"
	"testing"
)

func TestConflictPageRejectsInvalidBoundsBeforeStore(t *testing.T) {
	service := &service{}
	for _, input := range []struct{ page, size int }{{0, 10}, {-1, 10}, {1000001, 10}, {1, 0}, {1, 101}} {
		rows, total, err := service.Conflicts(context.Background(), 1, input.page, input.size)
		if err == nil || rows != nil || total != 0 {
			t.Fatalf("invalid page returned data: %+v %d %v", rows, total, err)
		}
	}
}
