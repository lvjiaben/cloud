// This file serializes merchant navigation configuration writes. Product
// rows are locked as one ordered set so concurrent membership changes cannot
// silently move a product into two merchant directories.
package productdirectory

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/service/audit"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

var referencePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,39}$`)

const (
	operationSave   = "product_directory.save"
	targetDirectory = "product_directory"
)

func validName(value string) bool {
	return value == strings.TrimSpace(value) && utf8.RuneCountInString(value) >= 1 && utf8.RuneCountInString(value) <= 128 && strings.IndexFunc(value, unicode.IsControl) < 0
}

// Save enforces ownership before accessing directory records.
func (s *serviceImpl) Save(ctx context.Context, input SaveInput) (out *Directory, boundaryErr error) {
	defer func() { boundaryErr = directoryError(boundaryErr) }()
	creating := input.DirectoryNo == ""
	permission := PermissionUpdate
	if creating {
		permission = PermissionCreate
	}
	actor, err := s.access.Admin(ctx, permission)
	if err != nil {
		return nil, err
	}
	input, err = prepareSave(input, creating)
	if err != nil {
		return nil, err
	}
	var result *Directory
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var before *Directory
		var beforeSnapshot any = struct{}{}
		if !creating {
			if loadErr := s.database.Model(do.ProductDirectory{}).Ctx(ctx).Where(do.ProductDirectory{TenantId: 0, DirectoryNo: input.DirectoryNo}).Fields(directoryFields).LockUpdate().Scan(&before); loadErr != nil {
				return loadErr
			}
			if before == nil {
				return bizerr.NewCode(CodeNotAccessible)
			}
			if before.Version != input.ExpectedVersion {
				return bizerr.NewCode(CodeConflict)
			}
			if loadErr := s.database.Model(do.ProductDirectoryMembership{}).Ctx(ctx).Where(do.ProductDirectoryMembership{TenantId: 0, DirectoryNo: input.DirectoryNo}).Fields("product_no").OrderAsc("product_no").Scan(&before.ProductNos); loadErr != nil {
				return loadErr
			}
			beforeSnapshot = before
		}
		if len(input.ProductNos) > 0 {
			var products []struct{ ProductNo string }
			if loadErr := s.database.Model(do.CatalogProduct{}).Ctx(ctx).Where(do.CatalogProduct{TenantId: 0}).WhereIn("product_no", input.ProductNos).Fields("product_no").OrderAsc("product_no").LockUpdate().Scan(&products); loadErr != nil {
				return loadErr
			}
			if len(products) != len(input.ProductNos) {
				return bizerr.NewCode(CodeNotAccessible)
			}
			conflicts, countErr := s.database.Model(do.ProductDirectoryMembership{}).Ctx(ctx).Where(do.ProductDirectoryMembership{TenantId: 0}).WhereIn("product_no", input.ProductNos).WhereNot("directory_no", input.DirectoryNo).Count()
			if countErr != nil {
				return countErr
			}
			if conflicts != 0 {
				return bizerr.NewCode(CodeConflict)
			}
		}
		data := do.ProductDirectory{TenantId: 0, DirectoryNo: input.DirectoryNo, NameZhCn: input.NameZhCn, NameEnUs: input.NameEnUs, SortOrder: input.SortOrder, StatusCode: input.StatusCode, UpdatedBy: actor}
		version := int64(1)
		if creating {
			data.CreatedBy = actor
			data.Version = version
			if _, writeErr := s.database.Model(do.ProductDirectory{}).Ctx(ctx).Data(data).Insert(); writeErr != nil {
				return writeErr
			}
		} else {
			version = before.Version + 1
			data.Version = version
			if _, writeErr := s.database.Model(do.ProductDirectory{}).Ctx(ctx).Where(do.ProductDirectory{TenantId: 0, DirectoryNo: input.DirectoryNo, Version: input.ExpectedVersion}).Data(data).Update(); writeErr != nil {
				return writeErr
			}
		}
		if _, writeErr := s.database.Model(do.ProductDirectoryMembership{}).Ctx(ctx).Where(do.ProductDirectoryMembership{TenantId: 0, DirectoryNo: input.DirectoryNo}).Delete(); writeErr != nil {
			return writeErr
		}
		if len(input.ProductNos) > 0 {
			links := make([]do.ProductDirectoryMembership, len(input.ProductNos))
			for i, no := range input.ProductNos {
				links[i] = do.ProductDirectoryMembership{TenantId: 0, DirectoryNo: input.DirectoryNo, ProductNo: no}
			}
			if _, writeErr := s.database.Model(do.ProductDirectoryMembership{}).Ctx(ctx).Data(links).Insert(); writeErr != nil {
				return writeErr
			}
		}
		result = &Directory{DirectoryNo: input.DirectoryNo, NameZhCn: input.NameZhCn, NameEnUs: input.NameEnUs, SortOrder: input.SortOrder, StatusCode: input.StatusCode, Version: version, ProductNos: input.ProductNos, ProductCount: len(input.ProductNos), Products: []Product{}}
		if len(input.ProductNos) > 0 {
			if loadErr := s.database.Model(do.CatalogProduct{}).Ctx(ctx).Where(do.CatalogProduct{TenantId: 0}).WhereIn("product_no", input.ProductNos).Fields("product_no,name_zh_cn,name_en_us").OrderAsc("product_no").Scan(&result.Products); loadErr != nil {
				return loadErr
			}
		}
		auditErr := s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{IdempotencyScope: operationSave, IdempotencyKey: input.DirectoryNo + ":" + string(input.StatusCode) + ":" + strconv.FormatInt(version, 10), OperationCode: operationSave, TargetType: targetDirectory, TargetNo: input.DirectoryNo, ResultCode: string(audit.ResultSucceeded), RiskCode: string(audit.RiskLow), RequiredPermission: permission, BeforeSnapshot: beforeSnapshot, AfterSnapshot: result})
		return auditErr
	})
	return result, err
}

// prepareSave validates and copies the bounded product set before transaction writes.
func prepareSave(input SaveInput, creating bool) (SaveInput, error) {
	if !validName(input.NameZhCn) || !validName(input.NameEnUs) || input.SortOrder < 0 || input.SortOrder > 1000000 || len(input.ProductNos) > 200 || !creating && (!referencePattern.MatchString(input.DirectoryNo) || input.DirectoryNo == AllDirectory || input.ExpectedVersion < 1) {
		return input, bizerr.NewCode(CodeInvalid)
	}
	if creating {
		input.StatusCode = StatusEnabled
		var token [16]byte
		if _, err := rand.Read(token[:]); err != nil {
			return input, bizerr.WrapCode(err, CodeUnavailable)
		}
		input.DirectoryNo = "pdr_" + hex.EncodeToString(token[:])
	}
	if input.StatusCode != StatusEnabled && input.StatusCode != StatusDisabled {
		return input, bizerr.NewCode(CodeInvalid)
	}
	input.ProductNos = append([]string(nil), input.ProductNos...)
	sort.Strings(input.ProductNos)
	for i, no := range input.ProductNos {
		if !referencePattern.MatchString(no) || i > 0 && no == input.ProductNos[i-1] {
			return input, bizerr.NewCode(CodeInvalid)
		}
	}
	return input, nil
}
