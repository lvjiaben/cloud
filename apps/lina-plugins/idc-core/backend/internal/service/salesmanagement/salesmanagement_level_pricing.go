// This file owns independently versioned tier-wide percentages. Public values
// describe the percentage charged; product discounts retain their existing semantics.
package salesmanagement

import (
	"context"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/util/guid"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

const statusCleared = "cleared"

// LevelPricing is a tier's independent price revision; nil percentage means unset.
type LevelPricing struct {
	LevelNo               string
	PercentageBasisPoints *int
	Version               int64
}

// GetLevelPricing requires pricing authorization even when no percentage was configured.
func (s *serviceImpl) GetLevelPricing(ctx context.Context, levelNo string) (LevelPricing, error) {
	if _, err := s.authorize(ctx, permissionPricing, false); err != nil {
		return LevelPricing{}, err
	}
	var level *entity.SalesAgentLevel
	if err := s.database.Model(dao.SalesAgentLevel.Table()).Ctx(ctx).Where(do.SalesAgentLevel{TenantId: 0, LevelNo: levelNo}).Scan(&level); err != nil {
		return LevelPricing{}, managementError(err)
	}
	if level == nil {
		return LevelPricing{}, bizerr.NewCode(codeInvalid)
	}
	row, err := s.currentLevelPrice(ctx, levelNo)
	if err != nil {
		return LevelPricing{}, managementError(err)
	}
	return levelPricingProjection(levelNo, row), nil
}

// SaveLevelPricing locks the tier and appends a revision; nil clears future quotes only.
func (s *serviceImpl) SaveLevelPricing(ctx context.Context, levelNo string, percentage *int, expectedVersion int64) (LevelPricing, error) {
	if _, err := s.authorize(ctx, permissionPricing, false); err != nil {
		return LevelPricing{}, err
	}
	if levelNo == "" || len(levelNo) > 128 || expectedVersion < 0 || (percentage != nil && (*percentage < 0 || *percentage > 10000)) {
		return LevelPricing{}, bizerr.NewCode(codeInvalid)
	}
	var result LevelPricing
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		row, err := s.database.Model(dao.SalesAgentLevel.Table()).Ctx(ctx).Where(do.SalesAgentLevel{TenantId: 0, LevelNo: levelNo}).LockUpdate().One()
		if err != nil {
			return err
		}
		if len(row) == 0 {
			return bizerr.NewCode(codeInvalid)
		}
		previous, err := s.currentLevelPrice(ctx, levelNo)
		if err != nil {
			return err
		}
		before := levelPricingProjection(levelNo, previous)
		if before.Version != expectedVersion {
			return bizerr.NewCode(codeConflict)
		}
		number := "AGP_" + guid.S()
		if previous != nil {
			number = previous.PriceNo
		}
		result = LevelPricing{LevelNo: levelNo, PercentageBasisPoints: percentage, Version: before.Version + 1}
		digest, err := snapshotDigest(result)
		if err != nil {
			return err
		}
		value, status := 0, statusCleared
		if percentage != nil {
			value, status = *percentage, statusActive
		}
		if _, err = s.database.Model(dao.SalesLevelPrice.Table()).Ctx(ctx).Data(do.SalesLevelPrice{TenantId: 0, PriceNo: number, LevelNo: levelNo, PercentageBasisPoints: value, StatusCode: status, Version: result.Version, ContentDigest: digest}).Insert(); err != nil {
			return err
		}
		return s.auditChange(ctx, permissionPricing, "level_price", number, "", result.Version, before, result)
	})
	return result, managementError(err)
}

func (s *serviceImpl) currentLevelPrice(ctx context.Context, levelNo string) (*entity.SalesLevelPrice, error) {
	var row *entity.SalesLevelPrice
	err := s.database.Model(dao.SalesLevelPrice.Table()).Ctx(ctx).Where(do.SalesLevelPrice{TenantId: 0, LevelNo: levelNo}).OrderDesc("version").Limit(1).Scan(&row)
	return row, err
}
func levelPricingProjection(levelNo string, row *entity.SalesLevelPrice) LevelPricing {
	result := LevelPricing{LevelNo: levelNo}
	if row != nil {
		result.Version = row.Version
		if row.StatusCode == statusActive {
			value := row.PercentageBasisPoints
			result.PercentageBasisPoints = &value
		}
	}
	return result
}
