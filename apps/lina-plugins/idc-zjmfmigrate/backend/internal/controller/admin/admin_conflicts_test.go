package admin

import (
	"context"
	v1 "lina-plugin-idc-zjmfmigrate/backend/api/admin/v1"
	"lina-plugin-idc-zjmfmigrate/backend/internal/service/migrate"
	"testing"
)

type conflictPageFixture struct {
	migrate.Service
	batch      int64
	page, size int
}

func (fixture *conflictPageFixture) Conflicts(_ context.Context, batch int64, page, size int) ([]migrate.Conflict, int, error) {
	fixture.batch, fixture.page, fixture.size = batch, page, size
	return []migrate.Conflict{{Id: 21, Summary: "Safe fixture conflict"}}, 25, nil
}
func TestConflictsPreservesAuthorizedTotalAndRequestedWindow(t *testing.T) {
	fixture := new(conflictPageFixture)
	result, err := NewWithServices(fixture, nil).Conflicts(context.Background(), &v1.ConflictsReq{Id: 42, Page: 3, PageSize: 10})
	if err != nil || result.Total != 25 || len(result.List) != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if fixture.batch != 42 || fixture.page != 3 || fixture.size != 10 {
		t.Fatalf("request redirected: %+v", fixture)
	}
}
