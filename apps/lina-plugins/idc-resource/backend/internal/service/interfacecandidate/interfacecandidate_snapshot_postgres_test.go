// This file proves shared option reads do not wait for a blocked supplier and
// remain isolated by current interface revision, authority and refresh lease.
package interfacecandidate

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/channelconfig"
)

type snapshotExtensions struct {
	catalogReadExtensions
	calls      atomic.Int32
	exactCalls atomic.Int32
	entered    chan struct{}
	release    chan struct{}
	fail       bool
	changed    bool
	payload    string
}

func (s *snapshotExtensions) GetProviderExact(ctx context.Context, input plugincap.BusinessExtensionProviderExactInput) (*capmodel.BusinessExtensionProviderEvidence, error) {
	s.exactCalls.Add(1)
	return s.catalogReadExtensions.GetProviderExact(ctx, input)
}

func (s *snapshotExtensions) InvokeProviderMethod(ctx context.Context, _ pluginhost.BusinessExtensionMethodInvokeInput) (*capmodel.BusinessExtensionMethodResult, error) {
	s.calls.Add(1)
	if s.entered != nil {
		close(s.entered)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-s.release:
		}
	}
	if s.fail {
		return nil, gerror.New("private upstream error must not be persisted")
	}
	if s.payload != "" {
		return &capmodel.BusinessExtensionMethodResult{Version: "v1", Payload: []byte(s.payload)}, nil
	}
	if s.changed {
		return &capmodel.BusinessExtensionMethodResult{Version: "v1", Payload: []byte(`{"fields":[{"fieldKey":"region","items":[{"value":"2","labelZhCN":"新地区","labelEnUS":"New region"}]}]}`)}, nil
	}
	return &capmodel.BusinessExtensionMethodResult{Version: "v1", Payload: []byte(`{"ignoredSecret":"fixture-ignored-private","SyncState":"fixture-unused-private","fields":[{"fieldKey":"region","items":[{"value":"1","labelZhCN":"陕西","labelEnUS":"Shaanxi","optionToken":"fixture-unused-private"}]}]}`)}, nil
}

func TestCandidateRelationshipSurvivesSharedSnapshotPostgreSQL(t *testing.T) {
	database := newSnapshotTestDatabase(t)
	ctx := t.Context()
	const interfaceNo = "pfi_11111111111111111111111111111111"
	const revision = "rci_11111111111111111111111111111111"
	digestA, digestB := strings.Repeat("a", 64), strings.Repeat("b", 64)
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_resource_channel VALUES(0,'CHN_catalog','server-api','idc-zjmfcloud','dynamic','v1','v1.0.0',?,?,'http://supplier.example','{}','enabled',NULL,'')`, digestA, digestB); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_resource_interface_channel_binding VALUES(0,?,'CHN_catalog','idc-zjmfcloud','v1.0.0','v1',ARRAY['configuration.candidates'],?,?,'idc-zjmfcloud','v1.0.0','v1',ARRAY['provision','query'],?,?,'accepting',?)`, interfaceNo, digestA, digestB, digestA, digestB, revision); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_resource_interface_configuration_revision VALUES(0,?,?,'CHN_catalog','http://supplier.example','{}','{}')`, revision, interfaceNo); err != nil {
		t.Fatal(err)
	}
	extensions := &snapshotExtensions{catalogReadExtensions: catalogReadExtensions{evidence: &capmodel.BusinessExtensionProviderEvidence{
		PluginID: "idc-zjmfcloud", Point: resourcecap.DynamicProviderPoint, ReleaseVersion: "v1.0.0", ContractVersion: "v1", ReleaseDigest: digestA, DeclarationDigest: digestB, AuthorizedMethods: []string{resourcecap.DynamicProviderMethodConfigurationCandidates},
	}}, payload: `{"Fields":[{"FieldKey":"line","Items":[{"Value":"7","LabelZhCN":"线路七","LabelEnUS":"Line Seven"}]}],"Relationships":[{"ParentFieldKey":"region","ChildFieldKey":"line","Links":[{"ParentValue":"2","ChildValue":"7"}]}],"IgnoredSecret":"must-not-persist"}`}
	reader := NewLazy()
	if err := reader.Bind(database, extensions, func(context.Context, channelconfig.Configuration, string, []resourcecap.SecretBinding) (pluginhost.ConfidentialOutboundHandler, func(), error) {
		return func(context.Context, *pluginhost.ConfidentialOutboundRequest) (*pluginhost.ConfidentialOutboundResult, error) {
			return nil, gerror.New("unexpected HTTP outside the isolated fixture")
		}, func() {}, nil
	}); err != nil {
		t.Fatal(err)
	}
	request := resourceinterfaceprojectioncap.CandidateRequest{InterfaceInstanceNo: interfaceNo, Method: resourcecap.DynamicProviderMethodConfigurationCandidates, Values: []byte(`{"schemaVersion":"v1","requests":[{"fieldKey":"line"}]}`)}
	if _, err := reader.ResolveCandidates(ctx, request); err != nil {
		t.Fatal(err)
	}
	if worked, err := reader.RunCandidateRefresh(ctx); err != nil || !worked {
		t.Fatal("isolated directory was not refreshed", err)
	}
	other := NewLazy()
	if err := other.Bind(database, extensions, reader.deps.outbound); err != nil {
		t.Fatal(err)
	}
	exactBeforeRead := extensions.exactCalls.Load()
	result, err := other.ResolveCandidates(ctx, request)
	if extensions.exactCalls.Load() != exactBeforeRead {
		t.Fatal("database snapshot read parsed provider evidence")
	}
	if err != nil || len(result.Relationships) != 1 || result.Relationships[0].Links[0].ParentValue != "2" || extensions.calls.Load() != 1 {
		t.Fatal("another instance lost relationships or repeated upstream reads", err)
	}
	persisted, err := database.GetValue(ctx, `SELECT result_payload::text FROM plugin_idc_resource_interface_candidate_snapshot`)
	if err != nil || strings.Contains(persisted.String(), "must-not-persist") || strings.Contains(persisted.String(), "IgnoredSecret") {
		t.Fatal("shared directory stored non-projected supplier fields", err)
	}
}

func newSnapshotTestDatabase(t *testing.T) gdb.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_RESOURCE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_RESOURCE_TEST_DSN")
	}
	ctx := context.WithoutCancel(t.Context())
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	var suffix [8]byte
	if _, err = rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	schema := "idc_option_snapshot_" + hex.EncodeToString(suffix[:])
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn + separator + "search_path=" + url.QueryEscape(schema)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(ctx); closeErr != nil {
			t.Error(closeErr)
		}
		if _, dropErr := admin.Exec(ctx, `DROP SCHEMA "`+schema+`" CASCADE`); dropErr != nil {
			t.Error(dropErr)
		}
		if closeErr := admin.Close(ctx); closeErr != nil {
			t.Error(closeErr)
		}
	})
	if _, err = database.Exec(ctx, catalogReadFixtureSQL); err != nil {
		t.Fatal(err)
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("fixture path unavailable")
	}
	ddl, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../../manifest/sql/040-commerce-runtime.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err = database.Exec(ctx, string(ddl)); err != nil {
			t.Fatal(err)
		}
	}
	return database
}

func TestCandidateSnapshotReadRemainsFastDuringSharedRefreshPostgreSQL(t *testing.T) {
	database := newSnapshotTestDatabase(t)
	ctx := t.Context()
	const interfaceNo = "pfi_11111111111111111111111111111111"
	const revision = "rci_11111111111111111111111111111111"
	digestA, digestB := strings.Repeat("a", 64), strings.Repeat("b", 64)
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_resource_channel VALUES(0,'CHN_catalog','server-api','idc-zjmfcloud','dynamic','v1','v1.0.0',?,?,'http://supplier.example','{}','enabled',NULL,'')`, digestA, digestB); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_resource_interface_channel_binding VALUES(0,?,'CHN_catalog','idc-zjmfcloud','v1.0.0','v1',ARRAY['configuration.candidates'],?,?,'idc-zjmfcloud','v1.0.0','v1',ARRAY['provision','query'],?,?,'accepting',?)`, interfaceNo, digestA, digestB, digestA, digestB, revision); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_resource_interface_configuration_revision VALUES(0,?,?,'CHN_catalog','http://supplier.example','{}','{}')`, revision, interfaceNo); err != nil {
		t.Fatal(err)
	}
	extensions := &snapshotExtensions{catalogReadExtensions: catalogReadExtensions{evidence: &capmodel.BusinessExtensionProviderEvidence{
		PluginID: "idc-zjmfcloud", Point: resourcecap.DynamicProviderPoint, ReleaseVersion: "v1.0.0", ContractVersion: "v1", ReleaseDigest: digestA, DeclarationDigest: digestB, AuthorizedMethods: []string{resourcecap.DynamicProviderMethodConfigurationCandidates},
	}}, entered: make(chan struct{}), release: make(chan struct{})}
	reader := NewLazy()
	if err := reader.Bind(database, extensions, func(context.Context, channelconfig.Configuration, string, []resourcecap.SecretBinding) (pluginhost.ConfidentialOutboundHandler, func(), error) {
		return func(context.Context, *pluginhost.ConfidentialOutboundRequest) (*pluginhost.ConfidentialOutboundResult, error) {
			return nil, gerror.New("unexpected direct HTTP")
		}, func() {}, nil
	}); err != nil {
		t.Fatal(err)
	}
	request := resourceinterfaceprojectioncap.CandidateRequest{InterfaceInstanceNo: interfaceNo, Method: resourcecap.DynamicProviderMethodConfigurationCandidates, Values: []byte(`{"schemaVersion":"v1","requests":[{"fieldKey":"region"}]}`)}
	secondReader := NewLazy()
	if err := secondReader.Bind(database, extensions, reader.deps.outbound); err != nil {
		t.Fatal(err)
	}
	read := func(expected int) *resourceinterfaceprojectioncap.CandidateResult {
		t.Helper()
		start := time.Now()
		result, err := reader.ResolveCandidates(ctx, request)
		if err != nil || result == nil || len(result.Fields) != 1 || len(result.Fields[0].Items) != expected || time.Since(start) > 2*time.Second {
			t.Fatalf("option page blocked or lost projection: result=%#v err=%v duration=%s", result, err, time.Since(start))
		}
		return result
	}
	if result := read(0); result.SyncState != resourceinterfaceprojectioncap.CandidateSyncPending {
		t.Fatal("cold supplier status was not pending")
	}
	if _, err := database.Exec(ctx, `UPDATE plugin_idc_resource_interface_candidate_snapshot SET last_error_code='supplier_read_failed'`); err != nil {
		t.Fatal(err)
	}
	if result := read(0); result.SyncState != resourceinterfaceprojectioncap.CandidateSyncFailed {
		t.Fatal("unavailable supplier was not reported")
	}
	if extensions.calls.Load() != 0 {
		t.Fatal("cold page called supplier synchronously")
	}
	done := make(chan error, 1)
	go func() { _, err := reader.RunCandidateRefresh(ctx); done <- err }()
	select {
	case <-extensions.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("refresh was not claimed")
	}
	read(0)
	if worked, err := secondReader.RunCandidateRefresh(ctx); worked || err != nil {
		t.Fatalf("another worker acquired the live shared lease: worked=%v err=%v", worked, err)
	}
	close(extensions.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	extensions.entered = nil
	read(1)
	// Repeated commercial form reads must remain local even when connector
	// discovery is deliberately observable. Timings are evidence, not a flaky
	// machine-dependent pass threshold; authority/remote call counts are strict.
	discoveryBefore, supplierBefore := extensions.exactCalls.Load(), extensions.calls.Load()
	timings := make([]time.Duration, 100)
	warmStart := time.Now()
	for index := range timings {
		start := time.Now()
		read(1)
		timings[index] = time.Since(start)
	}
	if extensions.exactCalls.Load() != discoveryBefore || extensions.calls.Load() != supplierBefore {
		t.Fatal("warm local directory performed plugin discovery or supplier work")
	}
	sort.Slice(timings, func(i, j int) bool { return timings[i] < timings[j] })
	t.Logf("100 warm local candidate reads total=%s p50=%s p95=%s plugin-discovery=0 supplier-invocations=0", time.Since(warmStart), timings[49], timings[94])
	fromOtherInstance, err := secondReader.ResolveCandidates(ctx, request)
	if err != nil || len(fromOtherInstance.Fields[0].Items) != 1 || extensions.calls.Load() != 1 {
		t.Fatal("another instance failed to reuse the durable projection")
	}
	request.Values = []byte(`{"schemaVersion":"v1","currentSelections":[{"cpu":16}],"requests":[{"fieldKey":"region"}]}`)
	read(1)
	if extensions.calls.Load() != 1 {
		t.Fatal("changing CPU repeated supplier region reads")
	}
	// Executable upgrades do not change the database-owned directory key.
	// Page reads stay local; an explicitly due refresh performs one provider read.
	for index, digest := range []string{strings.Repeat("c", 64), strings.Repeat("d", 64)} {
		oldKey, keyErr := database.GetValue(ctx, `SELECT request_key FROM plugin_idc_resource_interface_candidate_snapshot WHERE observed_at IS NOT NULL LIMIT 1`)
		if keyErr != nil {
			t.Fatal(keyErr)
		}
		extensions.evidence.ReleaseDigest = digest
		if _, updateErr := database.Exec(ctx, `UPDATE plugin_idc_resource_interface_candidate_snapshot SET refresh_after=CURRENT_TIMESTAMP-INTERVAL '1 day' WHERE request_key=?`, oldKey.String()); updateErr != nil {
			t.Fatal(updateErr)
		}
		if index == 0 {
			read(1)
		}
		if worked, refreshErr := reader.RunCandidateRefresh(ctx); refreshErr != nil || !worked {
			t.Fatalf("obsolete package migration failed: %v", refreshErr)
		}
		read(1)
		count, countErr := database.GetValue(ctx, `SELECT COUNT(*) FROM plugin_idc_resource_interface_candidate_snapshot`)
		if countErr != nil || count.Int() != 1 || extensions.calls.Load() != int32(index+2) {
			t.Fatal("package upgrade duplicated snapshots or repeated supplier refresh")
		}
	}
	// Changing the connection cannot reuse the previous revision's warm labels.
	if _, err := database.Exec(ctx, `UPDATE plugin_idc_resource_interface_channel_binding SET current_configuration_revision='rci_22222222222222222222222222222222'`); err != nil {
		t.Fatal(err)
	}
	read(0)
	if _, err := database.Exec(ctx, `UPDATE plugin_idc_resource_interface_channel_binding SET current_configuration_revision=?`, revision); err != nil {
		t.Fatal(err)
	}
	read(1)
	if _, err := database.Exec(ctx, `DELETE FROM plugin_idc_resource_interface_candidate_snapshot WHERE observed_at IS NULL`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `UPDATE plugin_idc_resource_interface_candidate_snapshot SET refresh_after=CURRENT_TIMESTAMP,observed_at=CURRENT_TIMESTAMP-INTERVAL '6 minutes'`); err != nil {
		t.Fatal(err)
	}
	extensions.fail = true
	if _, err := reader.RunCandidateRefresh(ctx); err != nil {
		t.Fatal(err)
	}
	if result := read(1); result.SyncState != resourceinterfaceprojectioncap.CandidateSyncRetained {
		t.Fatal("failed refresh did not report retained choices")
	}
	var state struct {
		LeaseUntil    *time.Time
		LastErrorCode string
	}
	if err := database.GetScan(ctx, &state, `SELECT lease_until,last_error_code FROM plugin_idc_resource_interface_candidate_snapshot`); err != nil || state.LeaseUntil != nil || state.LastErrorCode != "supplier_read_failed" {
		t.Fatalf("failed refresh leaked error or stranded claim: %#v err=%v", state, err)
	}
	if _, err := database.Exec(ctx, `UPDATE plugin_idc_resource_interface_candidate_snapshot SET observed_at=CURRENT_TIMESTAMP-INTERVAL '7 days'`); err != nil {
		t.Fatal(err)
	}
	// A prolonged outage cannot hide a previously learned directory.
	read(1)
	fromOtherInstance, err = secondReader.ResolveCandidates(ctx, request)
	if err != nil || len(fromOtherInstance.Fields[0].Items) != 1 {
		t.Fatal("another instance discarded last successful supplier options")
	}
	// A successful new directory replaces old choices rather than appending them.
	extensions.fail, extensions.changed = false, true
	if _, err := database.Exec(ctx, `UPDATE plugin_idc_resource_interface_candidate_snapshot SET refresh_after=CURRENT_TIMESTAMP`); err != nil {
		t.Fatal(err)
	}
	if worked, err := reader.RunCandidateRefresh(ctx); err != nil || !worked {
		t.Fatalf("new directory refresh not executed worked=%v err=%v", worked, err)
	}
	last, err := database.GetOne(ctx, `SELECT last_error_code, result_payload::text AS result FROM plugin_idc_resource_interface_candidate_snapshot`)
	if err != nil || last["last_error_code"].String() != "" {
		t.Fatalf("new directory refresh failed: row=%s err=%v", last.Json(), err)
	}
	read(1)
	updated, err := reader.ResolveCandidates(ctx, request)
	if err != nil || len(updated.Fields[0].Items) != 1 || updated.Fields[0].Items[0].Value != "2" {
		t.Fatal("successful refresh retained deleted directory choices")
	}
	if _, err := database.Exec(ctx, `UPDATE plugin_idc_resource_channel SET status_code='disabled'`); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.ResolveCandidates(ctx, request); err == nil {
		t.Fatal("disabled interface served cached supplier choices")
	}
	// The supplier failure's arbitrary text never appears in stored projection.
	stored, err := database.GetValue(ctx, `SELECT row_to_json(s)::text FROM plugin_idc_resource_interface_candidate_snapshot s`)
	if err != nil || !json.Valid([]byte(stored.String())) || strings.Contains(stored.String(), "private upstream error") || strings.Contains(stored.String(), "fixture-ignored-private") || strings.Contains(stored.String(), "fixture-unused-private") {
		t.Fatal("projection persistence failed its secret-free error boundary")
	}
}
