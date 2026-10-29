package catalogdraft

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
)

const catalogPublicationTestDSNEnvironment = "LINAPRO_IDC_CORE_TEST_DSN"

type catalogDraftTestAuditWriter struct{ err error }

func (w *catalogDraftTestAuditWriter) AppendBusinessTransition(
	context.Context,
	idccap.BusinessAuditTransition,
) error {
	return w.err
}

func (w *catalogDraftTestAuditWriter) AppendBusinessBatchTransition(
	context.Context,
	idccap.BusinessAuditBatchTransition,
) (idccap.BusinessAuditAppendResult, error) {
	return idccap.BusinessAuditAppendResult{}, w.err
}

type catalogProductPublicationTestAuthorizer struct{ actorUserID int }

func (a *catalogProductPublicationTestAuthorizer) AuthorizeProductPublication(
	context.Context,
	string,
) (int, error) {
	return a.actorUserID, nil
}

type catalogDraftTestAutomationAdmission struct{}

func (catalogDraftTestAutomationAdmission) WithPublicationAdmission(
	ctx context.Context,
	_ string,
	action func(context.Context) error,
) error {
	// Production admission owns the publication transaction; the fake must
	// preserve that contract so audit failures exercise genuine rollback.
	return dao.CatalogProduct.DB().Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		return action(txCtx)
	})
}

func configureCatalogDraftIntegrationDB(t *testing.T, ctx context.Context) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv(catalogPublicationTestDSNEnvironment))
	if dsn == "" {
		t.Skipf("set %s to run PostgreSQL catalog publication tests", catalogPublicationTestDSNEnvironment)
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("open catalog publication admin database: %v", err)
	}
	var suffix [8]byte
	if _, err = rand.Read(suffix[:]); err != nil {
		t.Fatalf("generate catalog publication schema: %v", err)
	}
	schema := "linapro_catalog_publication_test_" + hex.EncodeToString(suffix[:])
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatalf("create catalog publication schema: %v", err)
	}
	original := gdb.GetAllConfig()
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	scopedDSN := dsn + separator + "search_path=" + url.QueryEscape(schema)
	if err = gdb.SetConfig(gdb.Config{gdb.DefaultGroupName: gdb.ConfigGroup{{Link: scopedDSN}}}); err != nil {
		t.Fatalf("configure catalog publication test database: %v", err)
	}
	t.Cleanup(func() {
		if err := gdb.SetConfig(original); err != nil {
			t.Errorf("restore catalog publication test database config: %v", err)
		}
		if strings.HasPrefix(schema, "linapro_catalog_publication_test_") {
			if _, dropErr := admin.Exec(ctx, `DROP SCHEMA "`+schema+`" CASCADE`); dropErr != nil {
				t.Errorf("drop catalog publication schema: %v", dropErr)
			}
		} else {
			t.Errorf("refusing to drop unexpected catalog publication schema %q", schema)
		}
		_ = admin.Close(ctx)
	})
}

func (s catalogDraftTestAutomationAdmission) WithSaleStateAdmission(ctx context.Context, versionNo string, _ bool, action func(context.Context) error) error {
	return s.WithPublicationAdmission(ctx, versionNo, action)
}

// AuthorizeProductSave provides ordinary editing authority in this fixture.
func (a *catalogProductPublicationTestAuthorizer) AuthorizeProductSave(ctx context.Context, versionNo string) (int, error) {
	return a.AuthorizeProductPublication(ctx, versionNo)
}
