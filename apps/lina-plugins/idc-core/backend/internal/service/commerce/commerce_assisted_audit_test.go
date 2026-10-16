// This file binds the real audit owner to isolated assisted-commerce fixtures.
// Only authenticated identity and permission providers are test doubles.
package commerce

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/service/audit"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

type assistedAuditBizContext struct{ actor *assistedQuoteAuthorizerStub }

func (s assistedAuditBizContext) Current(context.Context) bizctxcap.CurrentContext {
	return bizctxcap.CurrentContext{UserID: s.actor.actor, TenantID: 0, DataScope: 2}
}

type assistedAuditPrincipal struct{ actor *assistedQuoteAuthorizerStub }

func (s assistedAuditPrincipal) Current(context.Context) (*principal.Audience, error) {
	return &principal.Audience{UserID: s.actor.actor, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive}, nil
}

func realAssistedAuditWriter(t *testing.T, ctx context.Context, database gdb.DB, actor *assistedQuoteAuthorizerStub) (idccap.BusinessAuditWriter, gdb.DB) {
	t.Helper()
	original := gdb.GetAllConfig()
	t.Cleanup(func() {
		if err := gdb.SetConfig(original); err != nil {
			t.Error(err)
		}
	})
	if err := gdb.SetConfig(gdb.Config{gdb.DefaultGroupName: gdb.ConfigGroup{*database.GetConfig()}}); err != nil {
		t.Fatal(err)
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve audit migrations")
	}
	for _, name := range []string{"005-audit-evidence-sealing.sql", "006-audit-replay-lookup-and-target-index.sql", "008-audit-keyset-filter-indexes.sql", "009-audit-customer-link.sql"} {
		content, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../../manifest/sql", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err = database.Exec(ctx, string(content)); err != nil {
			t.Fatal(err)
		}
	}
	grants := make(map[authz.PermissionKey]bool)
	for _, key := range []string{permissionAssistedQuoteCreate} {
		grants[authz.PermissionKey(key)] = true
	}
	owner, err := audit.New(assistedAuditBizContext{actor}, assistedAuditPrincipal{actor}, riskReviewAuthzStub{grants: grants}, audit.Config{
		ReservationLookupKey:  audit.IntegrityKey{ID: "lookup.v1", Secret: bytes.Repeat([]byte{11}, 32)},
		ActiveReplayDigestKey: audit.IntegrityKey{ID: "replay.v1", Secret: bytes.Repeat([]byte{12}, 32)},
		ActiveIntegrityKey:    audit.IntegrityKey{ID: "integrity.v1", Secret: bytes.Repeat([]byte{13}, 32)},
		MaximumSnapshotBytes:  32 * 1024, MaximumSnapshotDepth: 8, MaximumSnapshotNodes: 512,
		MaximumPageSize: 100, MaximumResponseBytes: 512 * 1024,
	})
	if err != nil {
		t.Fatal(err)
	}
	writer, err := audit.NewBusinessWriterAdapter(owner)
	if err != nil {
		t.Fatal(err)
	}
	scoped, err := gdb.Instance(gdb.DefaultGroupName)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := scoped.Close(ctx); err != nil {
			t.Error(err)
		}
	})
	return writer, scoped
}
