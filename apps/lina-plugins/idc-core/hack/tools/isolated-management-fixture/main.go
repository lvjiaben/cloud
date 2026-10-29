// This test-only tool models a lost local management receipt for exactly one
// newly created order in a named disposable database. It cannot target the
// main runtime, change provider tasks, or alter money and fulfillment facts.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/os/gcfg"
)

// fixture contains only the environment identity; authentication is not read.
type fixture struct{ Protocol, DatabaseName, Root, BaseURL, MockOrigin string }

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "isolated management fixture rejected:", err)
		os.Exit(1)
	}
	fmt.Println("One test-owned local notification marked for management repair")
}

// confined resolves symlinks before accepting any fixture/config path.
func confined(path, root string) (string, error) {
	actual, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("fixture path unavailable")
	}
	relative, err := filepath.Rel(root, actual)
	if err != nil || relative == ".." || filepath.IsAbs(relative) || len(relative) > 3 && relative[:3] == ".."+string(filepath.Separator) {
		return "", fmt.Errorf("fixture path outside temporary root")
	}
	return actual, nil
}

func run() error {
	if len(os.Args) != 4 {
		return fmt.Errorf("expected order, intent and entitlement identifiers")
	}
	reference := regexp.MustCompile(`^[A-Za-z0-9_-]{1,40}$`)
	for _, value := range os.Args[1:] {
		if !reference.MatchString(value) {
			return fmt.Errorf("invalid fixture identifier")
		}
	}
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("workspace unavailable")
	}
	// The command is run from this plugin module, never an arbitrary CWD.
	temp, err := filepath.EvalSymlinks(filepath.Join(cwd, "..", "..", "..", "temp"))
	if err != nil {
		return fmt.Errorf("temporary workspace unavailable")
	}
	descriptor, err := confined(os.Getenv("IDC_ISOLATED_COMMERCE_FIXTURE"), temp)
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(descriptor)
	if err != nil {
		return fmt.Errorf("fixture descriptor unavailable")
	}
	var target fixture
	if json.Unmarshal(raw, &target) != nil || target.Protocol != "idc-isolated-commerce-v1" || !regexp.MustCompile(`^cloud_nightly_[0-9a-f]{12}$`).MatchString(target.DatabaseName) {
		return fmt.Errorf("disposable fixture identity required")
	}
	if err = validateOrigins(target); err != nil {
		return err
	}
	root, err := confined(target.Root, temp)
	if err != nil {
		return err
	}
	config, err := confined(filepath.Join(root, "config.yaml"), root)
	if err != nil {
		return err
	}
	planBytes, err := os.ReadFile(filepath.Join(root, "plan.json"))
	if err != nil {
		return fmt.Errorf("runtime plan unavailable")
	}
	var plan struct{ DatabaseName, BaseURL string }
	if json.Unmarshal(planBytes, &plan) != nil || plan.DatabaseName != target.DatabaseName || plan.BaseURL != target.BaseURL {
		return fmt.Errorf("runtime plan mismatch")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	db, err := openFixtureDatabase(ctx, config, root)
	if err != nil {
		return err
	}
	defer db.Close(ctx)
	actual, err := db.GetValue(ctx, "SELECT current_database()")
	if err != nil || actual.String() != target.DatabaseName {
		return fmt.Errorf("database identity mismatch")
	}
	result, err := db.Exec(ctx, `UPDATE plugin_idc_core_entitlement_created_outbox event
 SET status_code='manual_attention',attempt_count=10,owner_reference='',receipt_evidence_digest='',last_error_code='owner_delivery_result_unknown',updated_at=CURRENT_TIMESTAMP
 FROM plugin_idc_core_product_fulfillment_intent intent,plugin_idc_core_service_entitlement entitlement,plugin_idc_core_commerce_order orders
 WHERE event.tenant_id=0 AND event.intent_no=? AND event.entitlement_no=? AND event.owner_id='idc-resource'
 AND event.status_code='delivered' AND event.lease_token='' AND event.lease_expires_at IS NULL
 AND intent.tenant_id=0 AND intent.intent_no=event.intent_no AND intent.customer_no=event.customer_no AND intent.entitlement_owner_id=event.owner_id
 AND intent.order_no=? AND intent.status_code='completed' AND intent.financially_settled_at IS NOT NULL
 AND entitlement.tenant_id=0 AND entitlement.entitlement_no=event.entitlement_no AND entitlement.source_intent_no=intent.intent_no AND entitlement.customer_no=intent.customer_no
 AND entitlement.entitlement_owner_id=event.owner_id AND entitlement.status_code='active'
 AND orders.tenant_id=0 AND orders.order_no=intent.order_no AND orders.status_code='paid'`, os.Args[2], os.Args[3], os.Args[1])
	if err != nil {
		return fmt.Errorf("test fixture transition failed")
	}
	affected, err := result.RowsAffected()
	if err != nil || affected != 1 {
		return fmt.Errorf("exactly one delivered test-owned notification required")
	}
	return nil
}

// validateOrigins excludes the main app and all non-loopback destinations.
func validateOrigins(target fixture) error {
	origin, err := url.Parse(target.BaseURL)
	if err != nil || origin.Scheme != "http" || origin.Hostname() != "127.0.0.1" || origin.Path != "" || origin.Port() == "" {
		return fmt.Errorf("isolated API origin required")
	}
	switch origin.Port() {
	case "6030", "6031", "6032", "6680":
		return fmt.Errorf("main installation excluded")
	}
	mock, err := url.Parse(target.MockOrigin)
	if err != nil || mock.Scheme != "https" || mock.Hostname() != "127.0.0.1" || mock.Port() == "" {
		return fmt.Errorf("loopback mock upstream required")
	}
	return nil
}

// openFixtureDatabase reads only the confined local runtime configuration.
func openFixtureDatabase(ctx context.Context, config, root string) (gdb.DB, error) {
	adapter, err := gcfg.NewAdapterFile(filepath.Base(config))
	if err != nil {
		return nil, fmt.Errorf("configuration adapter unavailable")
	}
	if err = adapter.SetPath(root); err != nil {
		return nil, fmt.Errorf("configuration path unavailable")
	}
	link, err := adapter.Get(ctx, "database.default.link")
	if err != nil {
		return nil, fmt.Errorf("database configuration unavailable")
	}
	connection, ok := link.(string)
	if !ok || connection == "" {
		return nil, fmt.Errorf("database link unavailable")
	}
	return gdb.New(gdb.ConfigNode{Link: connection})
}
