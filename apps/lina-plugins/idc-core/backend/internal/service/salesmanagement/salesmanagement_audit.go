// This file appends business audit evidence in the caller's transaction.
package salesmanagement

import (
	"context"
	"lina-plugin-idc-core/backend/cap/idccap"
	"reflect"
	"strconv"
)

func (s *serviceImpl) auditChange(ctx context.Context, permission, kind, number, customer string, version int64, before, after any) error {
	if before == nil || (reflect.ValueOf(before).Kind() == reflect.Pointer && reflect.ValueOf(before).IsNil()) {
		before = struct{}{}
	}
	return s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{IdempotencyScope: "sales_management", IdempotencyKey: number + ":" + strconv.FormatInt(version, 10), OperationCode: "sales_" + kind + "_save", TargetType: "sales_" + kind, TargetNo: number, CustomerNo: customer, ResultCode: idccap.BusinessAuditResultSucceeded, ReasonCode: "admin_update", RequiredPermission: permission, BeforeSnapshot: before, AfterSnapshot: after})
}
