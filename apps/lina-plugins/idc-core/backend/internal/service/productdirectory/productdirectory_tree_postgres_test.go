// This file verifies hierarchy validation, bounded tree context and unchanged
// product membership in isolated PostgreSQL schemas, including failed writes.
package productdirectory

import (
	"context"
	"lina-core/pkg/bizerr"
	"testing"
)

func TestDirectoryTreeHierarchyAndPagingPostgres(t *testing.T) {
	db, service, access, audit := newDirectoryFixture(t)
	ctx := context.Background()
	create := func(name, parent string, order int) *Directory {
		t.Helper()
		row, err := service.Save(ctx, SaveInput{NameZhCn: name, NameEnUs: name, SortOrder: order, ParentDirectoryNo: parent})
		if err != nil {
			t.Fatal(err)
		}
		return row
	}
	root := create("服务", "", 10)
	child := create("云服务器", root.DirectoryNo, 5)
	second := create("费用", "", 20)
	if _, err := db.Exec(ctx, "INSERT INTO plugin_idc_core_product_directory_membership(product_no,directory_no) VALUES('product-a',?)", child.DirectoryNo); err != nil {
		t.Fatal(err)
	}
	page, err := service.ListAdminTree(ctx, 2, 1)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 3 || len(page.Items) != 1 || page.Items[0].DirectoryNo != child.DirectoryNo || page.Items[0].ProductCount != 1 || len(page.Parents) != 1 || page.Parents[0].DirectoryNo != root.DirectoryNo {
		t.Fatalf("bounded row/context projection: %#v", page)
	}
	flat, err := service.ListAdmin(ctx, 1, 100)
	if err != nil || len(flat.Items) != 3 {
		t.Fatalf("flat candidates preserved: %#v %v", flat, err)
	}
	if len(flat.Parents) != 0 {
		t.Fatal("flat candidates must not add duplicate ancestor context")
	}
	invalid := []SaveInput{
		{NameZhCn: "三级", NameEnUs: "Third", ParentDirectoryNo: child.DirectoryNo},
		{DirectoryNo: root.DirectoryNo, NameZhCn: root.NameZhCn, NameEnUs: root.NameEnUs, ParentDirectoryNo: second.DirectoryNo, StatusCode: StatusEnabled, ExpectedVersion: root.Version},
		{DirectoryNo: child.DirectoryNo, NameZhCn: child.NameZhCn, NameEnUs: child.NameEnUs, ParentDirectoryNo: child.DirectoryNo, StatusCode: StatusEnabled, ExpectedVersion: child.Version},
		{NameZhCn: "不存在", NameEnUs: "Missing", ParentDirectoryNo: "pdr_missing"},
	}
	for _, input := range invalid {
		if _, err = service.Save(ctx, input); err == nil {
			t.Fatalf("invalid hierarchy accepted: %#v", input)
		}
	}
	moved, err := service.Save(ctx, SaveInput{DirectoryNo: child.DirectoryNo, NameZhCn: child.NameZhCn, NameEnUs: child.NameEnUs, Icon: "lucide:database-zap", ParentDirectoryNo: second.DirectoryNo, SortOrder: 8, StatusCode: StatusEnabled, ExpectedVersion: child.Version})
	if err != nil || moved.ParentDirectoryNo != second.DirectoryNo || moved.Version != child.Version+1 || moved.ProductCount != 1 {
		t.Fatalf("valid move: %#v %v", moved, err)
	}
	account, err := service.ListAccount(ctx, 1, 100, "")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range account.Items {
		if row.DirectoryNo == child.DirectoryNo {
			found = row.ParentDirectoryNo == second.DirectoryNo && row.ServiceCount == 1
		}
	}
	if !found {
		t.Fatal("customer projection lost hierarchy or isolated count")
	}
	audit.fail = true
	if _, err = service.Save(ctx, SaveInput{DirectoryNo: child.DirectoryNo, NameZhCn: "失败修改", NameEnUs: child.NameEnUs, ParentDirectoryNo: root.DirectoryNo, SortOrder: 99, StatusCode: StatusEnabled, ExpectedVersion: moved.Version}); err == nil {
		t.Fatal("audit failure accepted")
	}
	audit.fail = false
	current, err := service.ListAdmin(ctx, 1, 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range current.Items {
		if row.DirectoryNo == child.DirectoryNo && (row.ParentDirectoryNo != second.DirectoryNo || row.SortOrder != 8 || row.Version != moved.Version) {
			t.Fatalf("failed write changed current menu: %#v", row)
		}
	}
	if _, err = service.Save(ctx, SaveInput{DirectoryNo: second.DirectoryNo, NameZhCn: second.NameZhCn, NameEnUs: second.NameEnUs, StatusCode: StatusDisabled, ExpectedVersion: second.Version}); err != nil {
		t.Fatal(err)
	}
	account, err = service.ListAccount(ctx, 1, 100, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range account.Items {
		if row.DirectoryNo == second.DirectoryNo || row.DirectoryNo == child.DirectoryNo {
			t.Fatal("disabled branch was published")
		}
	}
	// Hiding a branch changes navigation only, never the customer's service ownership.
	owned, err := service.ListServices(ctx, child.DirectoryNo, 1, 20)
	if err != nil || len(owned.Items) != 1 {
		t.Fatalf("enabled child service access preserved: %#v %v", owned, err)
	}
	access.deny = true
	if _, err = service.ListAdminTree(ctx, 1, 20); err == nil {
		t.Fatal("tree bypassed staff permission")
	}
	if _, ok := bizerr.As(err); !ok {
		t.Fatal("permission failure lost structured error")
	}
}

// Concurrent attachments cannot commit a third level through stale parent reads.
func TestDirectoryConcurrentHierarchyPostgres(t *testing.T) {
	_, service, _, _ := newDirectoryFixture(t)
	ctx := context.Background()
	root, err := service.Save(ctx, SaveInput{NameZhCn: "父级", NameEnUs: "Parent"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := service.Save(ctx, SaveInput{NameZhCn: "其他", NameEnUs: "Other"})
	if err != nil {
		t.Fatal(err)
	}
	begin := make(chan struct{})
	results := make(chan error, 2)
	go func() {
		<-begin
		_, e := service.Save(ctx, SaveInput{DirectoryNo: root.DirectoryNo, NameZhCn: root.NameZhCn, NameEnUs: root.NameEnUs, ParentDirectoryNo: other.DirectoryNo, ExpectedVersion: root.Version, StatusCode: StatusEnabled})
		results <- e
	}()
	go func() {
		<-begin
		_, e := service.Save(ctx, SaveInput{NameZhCn: "子菜单", NameEnUs: "Child", ParentDirectoryNo: root.DirectoryNo})
		results <- e
	}()
	close(begin)
	success := 0
	for range 2 {
		if <-results == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("concurrent writes must serialize hierarchy validation: %d succeeded", success)
	}
	page, err := service.ListAdmin(ctx, 1, 100)
	if err != nil {
		t.Fatal(err)
	}
	parents := make(map[string]string, len(page.Items))
	for _, row := range page.Items {
		parents[row.DirectoryNo] = row.ParentDirectoryNo
	}
	for id, parent := range parents {
		if parent != "" && parents[parent] != "" {
			t.Fatalf("third level committed at %s", id)
		}
	}
}
