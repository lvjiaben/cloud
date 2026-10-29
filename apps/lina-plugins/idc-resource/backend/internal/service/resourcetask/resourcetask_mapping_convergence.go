// This file atomically converges successful provisioning tasks into authoritative resource mappings.

package resourcetask

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/dao"
	"lina-plugin-idc-resource/backend/internal/model/do"
	"lina-plugin-idc-resource/backend/internal/model/entity"
)

// convergeSuccessfulProvision creates or activates the exact owner mapping
// before a successful provisioning task can commit.
func convergeSuccessfulProvision(
	ctx context.Context,
	tx gdb.TX,
	task *entity.Task,
	externalResourceRef string,
	random io.Reader,
) error {
	if resourcecap.CapabilityCode(task.CapabilityCode) != resourcecap.CapabilityProvision {
		return nil
	}
	native, err := nativePurchaseMapping(task)
	if err != nil {
		return err
	}
	if !workerIDPattern.MatchString(task.LocalResourceNo) ||
		!workerIDPattern.MatchString(task.ChannelNo) ||
		(!native && !workerIDPattern.MatchString(task.PoolNo)) ||
		(resourcecap.ResourceQuery{ExternalResourceRef: externalResourceRef}).Validate() != nil {
		return gerror.New("successful provision resource mapping is invalid")
	}
	columns := dao.Mapping.Columns()
	var mapping entity.Mapping
	err = tx.Model(dao.Mapping.Table()).Safe().Ctx(ctx).
		Fields(
			columns.Id,
			columns.MappingNo,
			columns.LocalResourceNo,
			columns.ChannelNo,
			columns.PoolNo,
			columns.ExternalResourceRef,
			columns.StatusCode,
			columns.Version,
		).
		Where(do.Mapping{
			TenantId: idccap.DefaultTenantID, LocalResourceNo: task.LocalResourceNo,
		}).
		WhereNot(columns.StatusCode, mappingStatusReleased).
		LockUpdate().
		Scan(&mapping)
	if err != nil && !gerror.Is(err, sql.ErrNoRows) {
		return gerror.Wrap(err, "load provisioning resource mapping")
	}
	if mapping.LocalResourceNo == "" {
		return insertActiveProvisionMapping(ctx, tx, task, externalResourceRef, random)
	}
	if mapping.ChannelNo != task.ChannelNo || mapping.PoolNo != task.PoolNo {
		return gerror.New("provisioning resource mapping destination does not match")
	}
	if mapping.StatusCode == string(mappingStatusActive) {
		if mapping.ExternalResourceRef != externalResourceRef {
			return gerror.New("active resource mapping identity does not match")
		}
		return nil
	}
	if mapping.StatusCode != string(mappingStatusProvisioning) ||
		(mapping.ExternalResourceRef != "" && mapping.ExternalResourceRef != externalResourceRef) {
		return gerror.New("provisioning resource mapping cannot be activated")
	}
	result, err := tx.Model(dao.Mapping.Table()).Safe().Ctx(ctx).
		Where(do.Mapping{
			TenantId: idccap.DefaultTenantID, Id: mapping.Id, MappingNo: mapping.MappingNo,
			StatusCode: mappingStatusProvisioning, Version: mapping.Version,
		}).
		Data(do.Mapping{
			ExternalResourceRef: externalResourceRef,
			StatusCode:          mappingStatusActive,
			Version:             gdb.Raw(columns.Version + " + 1"),
		}).
		Update()
	if err != nil {
		return gerror.Wrap(err, "activate provisioning resource mapping")
	}
	return requireAffected(result, 1, "activate provisioning resource mapping")
}

// insertActiveProvisionMapping creates a mapping when no prior owner reservation exists.
func insertActiveProvisionMapping(
	ctx context.Context,
	tx gdb.TX,
	task *entity.Task,
	externalResourceRef string,
	random io.Reader,
) error {
	mappingNo, err := newMappingNumber(random)
	if err != nil {
		return err
	}
	var pool any = task.PoolNo
	if task.PoolNo == "" {
		pool = gdb.Raw("NULL")
	}
	_, err = tx.Model(dao.Mapping.Table()).Safe().Ctx(ctx).Data(do.Mapping{
		TenantId:            idccap.DefaultTenantID,
		MappingNo:           mappingNo,
		LocalResourceNo:     task.LocalResourceNo,
		ChannelNo:           task.ChannelNo,
		PoolNo:              pool,
		ExternalResourceRef: externalResourceRef,
		StatusCode:          mappingStatusActive,
	}).Insert()
	if err != nil {
		return gerror.Wrap(err, "insert active provision resource mapping")
	}
	return nil
}

// newMappingNumber creates an unpredictable owner resource mapping number.
func newMappingNumber(random io.Reader) (string, error) {
	var content [16]byte
	if _, err := io.ReadFull(random, content[:]); err != nil {
		return "", gerror.Wrap(err, "generate resource mapping number")
	}
	return "mapping_" + hex.EncodeToString(content[:]), nil
}

// nativePurchaseMapping permits a missing pool only for the explicit immutable
// native protocol arm. A prefix or empty pool alone cannot grant this exception.
func nativePurchaseMapping(task *entity.Task) (bool, error) {
	var facts struct {
		SaleImportProtocol productfulfillmentcap.SaleImportProtocol `json:"saleImportProtocol"`
	}
	if task.OwnerMetadata != "" {
		if err := json.Unmarshal([]byte(task.OwnerMetadata), &facts); err != nil {
			return false, gerror.New("provision owner metadata is invalid")
		}
	}
	if facts.SaleImportProtocol == "" {
		if strings.HasPrefix(task.LocalResourceNo, "NAT_") {
			return false, gerror.New("native mapping protocol is unavailable")
		}
		return false, nil
	}
	if !facts.SaleImportProtocol.NativePurchase() || !strings.HasPrefix(task.LocalResourceNo, "NAT_") || task.PoolNo != "" {
		return false, gerror.New("native mapping destination is invalid")
	}
	return true, nil
}
