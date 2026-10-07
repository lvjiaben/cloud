package serveroperation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
)

func (s *serviceImpl) UpdateCustomerMetadata(ctx context.Context,
	command CustomerMetadataCommand) (*CustomerMetadata, error) {
	command.Alias = strings.TrimSpace(command.Alias)
	command.Remark = strings.TrimSpace(command.Remark)
	if s == nil || s.database == nil || s.audit == nil || ctx == nil || ctx.Err() != nil ||
		!validReference(command.CustomerNo, 40) || !validReference(command.ServiceNo, 40) ||
		command.ExpectedVersion < 1 || !validCustomerMetadata(command.Alias, 128) ||
		!validCustomerMetadata(command.Remark, 1000) {
		return nil, ErrInvalidInput
	}
	var result *CustomerMetadata
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var current *struct {
			Alias   string `orm:"customer_alias"`
			Remark  string `orm:"customer_remark"`
			Version int64  `orm:"version"`
		}
		if err := s.database.GetScan(ctx, &current, `SELECT customer_alias,customer_remark,version
FROM plugin_idc_core_service_instance
WHERE tenant_id=0 AND service_no=? AND customer_no=? FOR UPDATE`,
			command.ServiceNo, command.CustomerNo); err != nil {
			return gerror.Wrap(err, "read customer server metadata")
		}
		if current == nil {
			return ErrServiceUnavailable
		}
		if current.Alias == command.Alias && current.Remark == command.Remark {
			result = &CustomerMetadata{Alias: current.Alias, Remark: current.Remark, Version: current.Version}
			return nil
		}
		if current.Version != command.ExpectedVersion {
			return ErrOperationConflict
		}
		nextVersion := current.Version + 1
		if _, err := s.database.Exec(ctx, `UPDATE plugin_idc_core_service_instance
SET customer_alias=?,customer_remark=?,version=?,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND service_no=? AND customer_no=? AND version=?`, command.Alias,
			command.Remark, nextVersion, command.ServiceNo, command.CustomerNo, current.Version); err != nil {
			return gerror.Wrap(err, "update customer server metadata")
		}
		if err := s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "server.customer_metadata.update",
			IdempotencyKey:   command.ServiceNo + ":" + strconv.FormatInt(nextVersion, 10),
			OperationCode:    "server.customer_metadata.update", TargetType: "server_service",
			TargetNo: command.ServiceNo, CustomerNo: command.CustomerNo, ResultCode: "succeeded",
			ReasonCode: "customer_request", BeforeSnapshot: map[string]any{
				"alias_digest": metadataDigest(current.Alias), "alias_empty": current.Alias == "",
				"remark_digest": metadataDigest(current.Remark), "remark_empty": current.Remark == "",
				"service_version": current.Version,
			}, AfterSnapshot: map[string]any{
				"alias_digest": metadataDigest(command.Alias), "alias_empty": command.Alias == "",
				"remark_digest": metadataDigest(command.Remark), "remark_empty": command.Remark == "",
				"service_version": nextVersion,
			},
		}); err != nil {
			return gerror.Wrap(err, "append customer server metadata audit")
		}
		result = &CustomerMetadata{Alias: command.Alias, Remark: command.Remark, Version: nextVersion}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func metadataDigest(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func (s *accountServiceImpl) UpdateCurrentCustomerMetadata(ctx context.Context, serviceNo,
	alias, remark string, expectedVersion int64) (*CustomerMetadata, error) {
	if s == nil || s.customers == nil || s.operations == nil || ctx == nil || ctx.Err() != nil {
		return nil, ErrInvalidInput
	}
	subject, err := s.customers.CurrentCustomer(ctx)
	if err != nil || subject == nil || !validReference(subject.CustomerNo, 40) {
		return nil, ErrServiceUnavailable
	}
	return s.operations.UpdateCustomerMetadata(ctx, CustomerMetadataCommand{
		CustomerNo: subject.CustomerNo, ServiceNo: serviceNo, Alias: alias, Remark: remark,
		ExpectedVersion: expectedVersion,
	})
}

func validCustomerMetadata(value string, maximum int) bool {
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > maximum {
		return false
	}
	for _, current := range value {
		if unicode.IsControl(current) {
			return false
		}
	}
	return true
}
