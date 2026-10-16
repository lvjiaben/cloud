// This file writes member navigation metadata without changing product ownership.
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
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

var iconPattern = regexp.MustCompile(`^[a-z0-9-]+:[a-z0-9-]+$`)

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
			before.ProductCount, err = s.database.Model(do.ProductDirectoryMembership{}).Ctx(ctx).Where(do.ProductDirectoryMembership{TenantId: 0, DirectoryNo: input.DirectoryNo}).Count()
			if err != nil {
				return err
			}
			beforeSnapshot = before
		}
		if before != nil && before.ProductCount > 0 && input.NavigationType != NavigationProduct {
			return bizerr.NewCode(CodeHasProducts)
		}
		data := do.ProductDirectory{TenantId: 0, DirectoryNo: input.DirectoryNo, NameZhCn: input.NameZhCn, NameEnUs: input.NameEnUs, SortOrder: input.SortOrder, StatusCode: input.StatusCode, NavigationType: string(input.NavigationType), TargetUrl: input.TargetURL, Icon: input.Icon, UpdatedBy: actor}
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
		productCount := 0
		if before != nil {
			productCount = before.ProductCount
		}
		result = &Directory{NavigationType: input.NavigationType, TargetURL: input.TargetURL, Icon: input.Icon, DirectoryNo: input.DirectoryNo, NameZhCn: input.NameZhCn, NameEnUs: input.NameEnUs, SortOrder: input.SortOrder, StatusCode: input.StatusCode, Version: version, ProductCount: productCount}
		auditErr := s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{IdempotencyScope: operationSave, IdempotencyKey: input.DirectoryNo + ":" + string(input.StatusCode) + ":" + strconv.FormatInt(version, 10), OperationCode: operationSave, TargetType: targetDirectory, TargetNo: input.DirectoryNo, ResultCode: string(audit.ResultSucceeded), RiskCode: string(audit.RiskLow), RequiredPermission: permission, BeforeSnapshot: beforeSnapshot, AfterSnapshot: result})
		return auditErr
	})
	return result, err
}

// prepareSave validates navigation metadata before transaction writes.
func prepareSave(input SaveInput, creating bool) (SaveInput, error) {
	var err error
	input, err = prepareNavigation(input)
	if err != nil {
		return input, err
	}
	if !validName(input.NameZhCn) || !validName(input.NameEnUs) || input.SortOrder < 0 || input.SortOrder > 1000000 || !creating && (!referencePattern.MatchString(input.DirectoryNo) || input.DirectoryNo == AllDirectory || input.ExpectedVersion < 1) {
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
	return input, nil
}

// prepareNavigation validates destination type and navigation metadata.
func prepareNavigation(input SaveInput) (SaveInput, error) {
	if input.NavigationType == "" {
		input.NavigationType = NavigationProduct
	}
	if input.Icon == "" {
		input.Icon = "lucide:folder"
	}
	if !iconPattern.MatchString(input.Icon) || len(input.Icon) > 80 {
		return input, bizerr.NewCode(CodeInvalid)
	}
	switch input.NavigationType {
	case NavigationProduct:
		input.TargetURL = ""
	case NavigationSystem, NavigationPlugin:
		parsed, err := url.ParseRequestURI(input.TargetURL)
		if err != nil || parsed.IsAbs() || !strings.HasPrefix(input.TargetURL, "/") || strings.HasPrefix(input.TargetURL, "//") {
			return input, bizerr.NewCode(CodeInvalid)
		}
	case NavigationCustom:
		parsed, err := url.ParseRequestURI(input.TargetURL)
		if err != nil || parsed.Host == "" || parsed.User != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") {
			return input, bizerr.NewCode(CodeInvalid)
		}
	default:
		return input, bizerr.NewCode(CodeInvalid)
	}
	if len(input.TargetURL) > 2048 {
		return input, bizerr.NewCode(CodeInvalid)
	}

	return input, nil
}
