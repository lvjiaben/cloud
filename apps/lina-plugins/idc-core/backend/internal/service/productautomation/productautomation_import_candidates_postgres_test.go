// This file verifies immutable imported candidate evidence and atomic rejection
// of changed tokens in a self-contained PostgreSQL schema.

package productautomation

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
)

func TestImportedCandidateEvidencePostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if e := admin.Close(ctx); e != nil {
			t.Error(e)
		}
	})
	schema := newAutomationSchema(t)
	if _, err = admin.Exec(ctx, `CREATE SCHEMA `+quoteAutomationIdentifier(schema)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, e := admin.Exec(ctx, `DROP SCHEMA `+quoteAutomationIdentifier(schema)+` CASCADE`); e != nil {
			t.Error(e)
		}
	})
	db := openAutomationDatabase(t, ctx, dsn, schema)
	if _, err = db.Exec(ctx, `CREATE TABLE plugin_idc_core_catalog_import_candidate(id BIGSERIAL PRIMARY KEY,tenant_id INT NOT NULL DEFAULT 0,version_no VARCHAR(40) NOT NULL,field_key VARCHAR(64) NOT NULL,candidate_value VARCHAR(256) NOT NULL,option_token VARCHAR(4096) NOT NULL,created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,UNIQUE(version_no,field_key,candidate_value))`); err != nil {
		t.Fatal(err)
	}
	version := "ver_" + strings.Repeat("a", 32)
	queries := []configurationcontract.DynamicCandidateQuery{{FieldKey: "region"}}
	result := &resourceinterfaceprojectioncap.CandidateResult{Fields: []resourceinterfaceprojectioncap.CandidateField{{FieldKey: "region", Items: []resourceinterfaceprojectioncap.Candidate{{Value: "root", OptionToken: "token-root", LabelZhCN: "区域", LabelEnUS: "Region"}, {Value: "child", ParentValue: "root", OptionToken: "token-child", LabelZhCN: "机房", LabelEnUS: "Datacenter"}}}}}
	for i := 0; i < 2; i++ {
		got, e := importedCandidateResult(ctx, db, version, queries, result)
		if e != nil || len(got["region"]) != 2 {
			t.Fatalf("persist/replay: %+v %v", got, e)
		}
		if got["region"][1].ParentValue != "root" {
			t.Fatalf("parent lost: %+v", got)
		}
	}
	result.Fields[0].Items[1].OptionToken = "token-replaced"
	result.Fields[0].Items = append(result.Fields[0].Items, resourceinterfaceprojectioncap.Candidate{Value: "new", OptionToken: "token-new", LabelZhCN: "新增", LabelEnUS: "New"})
	if _, e := importedCandidateResult(ctx, db, version, queries, result); e == nil {
		t.Fatal("changed token accepted")
	}
	if count, e := db.Model(do.CatalogImportCandidate{}).Ctx(ctx).Count(); e != nil || count != 2 {
		t.Fatalf("failed response left evidence: %d %v", count, e)
	}
	result.Fields[0].Items[1].OptionToken = "token-child"
	result.Fields[0].Items[0].ParentValue = "child"
	if _, e := importedCandidateResult(ctx, db, version, queries, result); e == nil {
		t.Fatal("candidate cycle accepted")
	}
	if _, e := importedCandidateResult(ctx, db, version, queries, &resourceinterfaceprojectioncap.CandidateResult{}); e == nil {
		t.Fatal("missing field accepted")
	}
}
