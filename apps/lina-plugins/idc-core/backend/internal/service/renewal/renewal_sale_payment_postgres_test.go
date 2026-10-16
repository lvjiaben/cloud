// This file verifies the migrated renewal payment gate independently of quote
// orchestration. The real database trigger blocks direct invoice payment and
// preserves paid facts; transaction-local projections isolate financial data.

package renewal

import (
	"context"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
)

func TestStoppedRenewalDirectPaymentPostgreSQL(t *testing.T) {
	ctx, database := newRenewalPostgreSQLDatabase(t)
	err := database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if _, err := database.Exec(ctx, `
CREATE TEMP TABLE plugin_idc_core_catalog_product ON COMMIT DROP AS SELECT
 0 AS tenant_id,'product'::text AS product_no,'active'::text AS status_code,NULL::timestamptz AS deleted_at;
CREATE TEMP TABLE plugin_idc_core_service_entitlement ON COMMIT DROP AS SELECT
 0 AS tenant_id,'entitlement'::text AS entitlement_no,'customer'::text AS customer_no,'product'::text AS product_no;
CREATE TEMP TABLE plugin_idc_core_business_operation ON COMMIT DROP AS SELECT
 0 AS tenant_id,'entitlement'::text AS subject_type_code,'entitlement'::text AS subject_no,
 'customer'::text AS customer_no,'customer'::text AS payer_customer_no,'invoice'::text AS invoice_no,'renewal'::text AS operation_type_code;
CREATE TEMP TABLE plugin_idc_core_billing_invoice ON COMMIT DROP AS SELECT
 0 AS tenant_id,'invoice'::text AS invoice_no,'customer'::text AS customer_no,'renewal'::text AS source_code,
 0::bigint AS paid_amount_minor,0::bigint AS credit_amount_minor;
CREATE TRIGGER trg_idc_renewal_sale_payment
 BEFORE UPDATE OF paid_amount_minor,credit_amount_minor ON plugin_idc_core_billing_invoice
 FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_renewal_sale_payment();
`); err != nil {
			return err
		}
		for _, column := range []string{"paid_amount_minor", "credit_amount_minor"} {
			for _, stopped := range []bool{true, false} {
				status := "active"
				if stopped {
					status = "suspended"
				}
				if _, err := database.Exec(ctx, `UPDATE plugin_idc_core_catalog_product SET status_code=?`, status); err != nil {
					return err
				}
				// A savepoint restores the transaction after expected constraint rejection.
				updateErr := database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
					_, err := database.Exec(ctx, "UPDATE plugin_idc_core_billing_invoice SET "+column+"=100")
					return err
				})
				if stopped && updateErr == nil || !stopped && updateErr != nil {
					t.Fatalf("column=%s stopped=%v payment error=%v", column, stopped, updateErr)
				}
				value, err := database.GetValue(ctx, "SELECT "+column+" FROM plugin_idc_core_billing_invoice")
				want := int64(100)
				if stopped {
					want = 0
				}
				if err != nil || value.Int64() != want {
					t.Fatalf("column=%s stopped=%v value=%v err=%v", column, stopped, value, err)
				}
			}
		}
		// Stopping later does not erase paid/credit facts or reject no-op replay.
		if _, err := database.Exec(ctx, `UPDATE plugin_idc_core_catalog_product SET status_code='suspended';
UPDATE plugin_idc_core_billing_invoice SET paid_amount_minor=paid_amount_minor,credit_amount_minor=credit_amount_minor;`); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
