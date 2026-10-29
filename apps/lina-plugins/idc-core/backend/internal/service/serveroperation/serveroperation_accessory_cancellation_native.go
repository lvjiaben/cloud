// Native cancellation projects only an owner-proven unpaid closure onto its explicit entitlement intent.

package serveroperation

import (
	"context"
	"github.com/gogf/gf/v2/database/gdb"
)

const nativeAccessoryCancellationSQL = `SELECT op.operation_no,COALESCE(op.service_no,'') AS service_no,
op.subject_type_code,op.subject_no,op.customer_no,op.invoice_no,op.quote_no,op.currency_code,
op.status_code,COALESCE(op.settlement_no,'') AS settlement_no,op.financially_settled_at,
op.execution_started_at,op.financial_settlement_required,op.version
FROM plugin_idc_core_business_operation op
JOIN plugin_idc_core_service_entitlement owner ON owner.tenant_id=op.tenant_id AND owner.entitlement_no=op.subject_no
 AND owner.customer_no=op.customer_no AND owner.currency_code=op.currency_code
 AND owner.entitlement_owner_id='idc-resource' AND owner.upstream_service_no=''
JOIN plugin_idc_core_server_accessory_quote quote ON quote.tenant_id=op.tenant_id AND quote.quote_no=op.quote_no
 AND quote.subject_type_code=op.subject_type_code AND quote.subject_no=op.subject_no
 AND quote.customer_no=op.customer_no AND quote.currency_code=op.currency_code
WHERE op.tenant_id=0 AND op.order_no=? AND op.operation_type_code='addon'
 AND op.subject_type_code='entitlement' AND op.service_no IS NULL AND quote.service_no IS NULL
LIMIT 2 FOR UPDATE OF op,owner`

// accessoryCancellationQuery establishes native parent locks before selecting
// the operation, while preserving the original legacy query unchanged.
func accessoryCancellationQuery(ctx context.Context, database gdb.DB, orderNo string) (string, error) {
	native, err := isNativeAccessoryOrder(ctx, database, orderNo)
	if err != nil {
		return "", err
	}
	if !native {
		return accessoryCancellationSQL, nil
	}
	if err = lockNativeAccessoryParent(ctx, database, orderNo); err != nil {
		return "", err
	}
	return nativeAccessoryCancellationSQL, nil
}
