package backend

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
)

func TestChannelLifecycleMigrationsAndInsertContractsPostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv(catalogIntegrationDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run channel lifecycle PostgreSQL tests", catalogIntegrationDSNEnv)
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	schema := newBillingIntegrationSchema(t)
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoteCatalogIdentifier(schema)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(ctx, "DROP SCHEMA "+quoteCatalogIdentifier(schema)+" CASCADE")
		_ = admin.Close(ctx)
	})
	database, err := gdb.New(gdb.ConfigNode{Link: appendCatalogDSNParameter(
		dsn, "search_path="+url.QueryEscape(schema),
	)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close(ctx) })
	if _, err = database.Exec(ctx, catalogDictionaryFixtureSQL); err != nil {
		t.Fatal(err)
	}

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve channel lifecycle migration path")
	}
	migrationRoot := filepath.Join(filepath.Dir(filename), "..", "manifest", "sql")
	paths, err := filepath.Glob(filepath.Join(migrationRoot, "[0-9][0-9][0-9]-*.sql"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(paths)
	for _, path := range paths {
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if _, err = database.Exec(ctx, string(content)); err != nil {
			t.Fatalf("execute %s: %v", filepath.Base(path), err)
		}
	}
	for _, name := range []string{"103-channel-renewal-payer.sql", "104-channel-service-operation.sql",
		"105-channel-priced-service-operation.sql"} {
		content, readErr := os.ReadFile(filepath.Join(migrationRoot, name))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if _, err = database.Exec(ctx, string(content)); err != nil {
			t.Fatalf("replay %s: %v", name, err)
		}
	}

	for index, query := range []string{channelBusinessOperationInsertSQL, channelServiceOperationInsertSQL,
		channelPricedBusinessOperationInsertSQL, channelPricedServiceOperationInsertSQL,
		channelPricedQuoteInsertSQL} {
		statement := "channel_lifecycle_insert_" + strconv.Itoa(index+1)
		if _, err = database.Exec(ctx, "PREPARE "+statement+" AS "+channelNumberedParameters(query)); err != nil {
			t.Fatalf("prepare channel lifecycle insert %d: %v", index+1, err)
		}
		if _, err = database.Exec(ctx, "DEALLOCATE "+statement); err != nil {
			t.Fatal(err)
		}
	}
}

func channelNumberedParameters(query string) string {
	var result strings.Builder
	parameter := 0
	for _, character := range query {
		if character == '?' {
			parameter++
			result.WriteByte('$')
			result.WriteString(strconv.Itoa(parameter))
			continue
		}
		result.WriteRune(character)
	}
	return result.String()
}
