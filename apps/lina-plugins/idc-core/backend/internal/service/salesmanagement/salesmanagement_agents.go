// This file assigns customer tiers and keeps a durable qualification history.
// A customer row serializes first assignment and subsequent optimistic updates.
package salesmanagement

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/util/guid"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

func (s *serviceImpl) ListAgents(ctx context.Context, input PageInput) (AgentPage, error) {
	result := AgentPage{Items: []Agent{}}
	if _, err := s.authorize(ctx, permissionOverview, true); err != nil {
		return result, err
	}
	input, err := normalizePage(input)
	if err != nil {
		return result, err
	}
	query := s.database.Model(dao.SalesAgentQualification.Table()+" q").Ctx(ctx).
		LeftJoin(dao.CustomerSubject.Table()+" c", "c.tenant_id=q.tenant_id AND c.customer_no=q.customer_no").
		LeftJoin(dao.SalesAgentLevel.Table()+" l", "l.tenant_id=q.tenant_id AND l.level_no=q.level_no").Where("q.tenant_id", 0)
	if input.Search != "" {
		query = query.Where("c.display_name LIKE ? OR CAST(c.customer_code AS TEXT) LIKE ?", "%"+input.Search+"%", "%"+input.Search+"%")
	}
	result.Total, err = query.Clone().Count()
	if err != nil {
		return result, managementError(err)
	}
	rows, err := query.Fields("q.customer_no,c.customer_code,c.display_name,q.level_no,l.name_zh_cn AS level_name,q.status_code,q.expires_at,q.version").OrderDesc("q.id").Page(input.Page, input.PageSize).All()
	if err != nil {
		return result, managementError(err)
	}
	for _, row := range rows {
		item := Agent{CustomerNo: row["customer_no"].String(), CustomerCode: row["customer_code"].String(), CustomerName: row["display_name"].String(), LevelNo: row["level_no"].String(), LevelName: row["level_name"].String(), Status: row["status_code"].String(), Version: row["version"].Int64()}
		if !row["expires_at"].IsNil() {
			v := row["expires_at"].Time()
			item.ExpiresAt = &v
		}
		result.Items = append(result.Items, item)
	}
	return result, nil
}
func (s *serviceImpl) SaveAgent(ctx context.Context, input AgentInput) (Agent, error) {
	actor, err := s.authorize(ctx, permissionQualification, true)
	if err != nil {
		return Agent{}, err
	}
	now := time.Now().UTC()
	if input.CustomerNo == "" || input.LevelNo == "" || (input.Status != statusActive && input.Status != statusSuspended) || (input.ExpiresAt != nil && !input.ExpiresAt.After(now)) {
		return Agent{}, bizerr.NewCode(codeInvalid)
	}
	var result Agent
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var customer *entity.CustomerSubject
		if err := s.database.Model(dao.CustomerSubject.Table()).Ctx(ctx).Where(do.CustomerSubject{TenantId: 0, CustomerNo: input.CustomerNo}).LockUpdate().Scan(&customer); err != nil {
			return err
		}
		if customer == nil || customer.StatusCode == "anonymized" || customer.StatusCode == "disabled" {
			return bizerr.NewCode(codeInvalid)
		}
		var level *entity.SalesAgentLevel
		if err := s.database.Model(dao.SalesAgentLevel.Table()).Ctx(ctx).Where(do.SalesAgentLevel{TenantId: 0, LevelNo: input.LevelNo, StatusCode: statusActive}).Scan(&level); err != nil {
			return err
		}
		if level == nil {
			return bizerr.NewCode(codeInvalid)
		}
		var current *entity.SalesAgentQualification
		if err := s.database.Model(dao.SalesAgentQualification.Table()).Ctx(ctx).Where(do.SalesAgentQualification{TenantId: 0, CustomerNo: input.CustomerNo}).OrderDesc("id").LockUpdate().Scan(&current); err != nil {
			return err
		}
		version := int64(1)
		number := "AQL_" + guid.S()
		event := do.SalesAgentQualificationEvent{TenantId: 0, EventNo: "AQE_" + guid.S(), ToLevelNo: input.LevelNo, ToStatusCode: input.Status, ReasonCode: "admin_update", ActorReference: strconv.Itoa(actor), OccurredAt: &now}
		if current == nil {
			if input.ExpectedVersion != 0 {
				return bizerr.NewCode(codeConflict)
			}
			if _, err := s.database.Model(dao.SalesAgentQualification.Table()).Ctx(ctx).Data(do.SalesAgentQualification{TenantId: 0, QualificationNo: number, CustomerNo: input.CustomerNo, LevelNo: input.LevelNo, StatusCode: input.Status, EffectiveAt: &now, ExpiresAt: input.ExpiresAt, Version: version}).Insert(); err != nil {
				return err
			}
		} else {
			if current.Version != input.ExpectedVersion {
				return bizerr.NewCode(codeConflict)
			}
			number = current.QualificationNo
			version = current.Version + 1
			event.FromLevelNo = current.LevelNo
			event.FromStatusCode = current.StatusCode
			if _, err := s.database.Model(dao.SalesAgentQualification.Table()).Ctx(ctx).Where(do.SalesAgentQualification{TenantId: 0, QualificationNo: number, Version: current.Version}).Data(do.SalesAgentQualification{LevelNo: input.LevelNo, StatusCode: input.Status, ExpiresAt: input.ExpiresAt, Version: version}).Update(); err != nil {
				return err
			}
			// A nil DO time is omitted by ORM; explicitly clear the optional expiry.
			if input.ExpiresAt == nil {
				if _, err := s.database.Model(dao.SalesAgentQualification.Table()).Ctx(ctx).Where(do.SalesAgentQualification{TenantId: 0, QualificationNo: number}).Data("expires_at", gdb.Raw("NULL")).Update(); err != nil {
					return err
				}
			}
		}
		result = Agent{CustomerNo: input.CustomerNo, CustomerCode: strconv.FormatInt(customer.CustomerCode, 10), CustomerName: customer.DisplayName, LevelNo: input.LevelNo, LevelName: level.NameZhCn, Status: input.Status, ExpiresAt: input.ExpiresAt, Version: version}
		digest, err := snapshotDigest(result)
		if err != nil {
			return err
		}
		event.QualificationNo = number
		event.EventSequence = version
		event.EvidenceDigest = digest
		if _, err = s.database.Model(dao.SalesAgentQualificationEvent.Table()).Ctx(ctx).Data(event).Insert(); err != nil {
			return err
		}
		return s.auditChange(ctx, permissionQualification, "agent", number, input.CustomerNo, version, current, result)
	})
	return result, managementError(err)
}
func snapshotDigest(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}
