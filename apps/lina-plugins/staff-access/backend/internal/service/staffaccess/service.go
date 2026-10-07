package staffaccess

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/sessioncap"
	"lina-core/pkg/plugin/capability/usercap"
	"lina-core/pkg/statusflag"
	"lina-plugin-staff-access/backend/cap/staffaccesscap"
)

const recentLoginWindow = 5 * time.Minute

// grantCountRow keeps aggregate scans compatible with the plugin database host,
// which accepts struct or slice destinations rather than primitive pointers.
type grantCountRow struct {
	Total int64 `json:"total"`
}

type Service struct {
	database      gdb.DB
	business      bizctxcap.Service
	users         usercap.Service
	authorization authz.Service
	sessions      sessioncap.Service
	now           func() time.Time
}

func New(database gdb.DB, business bizctxcap.Service, users usercap.Service, authorization authz.Service, sessions sessioncap.Service) (*Service, error) {
	if database == nil || business == nil || users == nil || authorization == nil || sessions == nil {
		return nil, gerror.New("staff access dependencies are unavailable")
	}
	return &Service{database: database, business: business, users: users, authorization: authorization, sessions: sessions, now: time.Now}, nil
}

type boundClient struct {
	service *Service
	caller  string
}

func Bind(service *Service, caller string) (staffaccesscap.Service, error) {
	if service == nil || strings.TrimSpace(caller) == "" {
		return nil, gerror.New("staff access source binding is invalid")
	}
	return &boundClient{service: service, caller: caller}, nil
}

func (c *boundClient) Check(ctx context.Context, input staffaccesscap.CheckRequest) (*staffaccesscap.Decision, error) {
	results, err := c.BatchCheck(ctx, staffaccesscap.BatchCheckRequest{UserIDs: []int64{input.UserID}, CapabilityCode: input.CapabilityCode, ScopeType: input.ScopeType, ScopeKey: input.ScopeKey})
	if err != nil {
		return nil, err
	}
	return &results[0], nil
}

func (c *boundClient) BatchCheck(ctx context.Context, input staffaccesscap.BatchCheckRequest) ([]staffaccesscap.Decision, error) {
	if c == nil || c.service == nil || strings.TrimSpace(c.caller) == "" || len(input.UserIDs) == 0 || len(input.UserIDs) > staffaccesscap.MaximumBatchSize {
		return nil, gerror.New("staff access batch is invalid")
	}
	seen := make(map[int64]struct{}, len(input.UserIDs))
	ids := make([]usercap.UserID, 0, len(input.UserIDs))
	for _, id := range input.UserIDs {
		if err := staffaccesscap.Validate(id, input.CapabilityCode, input.ScopeType, input.ScopeKey); err != nil {
			return nil, err
		}
		if _, exists := seen[id]; exists {
			return nil, gerror.New("staff access batch contains duplicate users")
		}
		seen[id] = struct{}{}
		ids = append(ids, usercap.UserID(strconv.FormatInt(id, 10)))
	}
	visible, err := c.service.users.BatchGet(ctx, ids)
	if err != nil {
		return nil, gerror.Wrap(err, "load target staff")
	}
	active := make(map[int64]bool, len(visible.Items))
	for _, item := range visible.Items {
		id, parseErr := strconv.ParseInt(string(item.ID), 10, 64)
		if parseErr == nil && item.Status == statusflag.EnabledValue {
			active[id] = true
		}
	}
	eligible := make(map[int64]bool, len(input.UserIDs))
	rows := []struct {
		UserID int64 `json:"user_id"`
	}{}
	query := `SELECT DISTINCT user_id FROM plugin_staff_access_grant WHERE tenant_id=0 AND status=1 AND (valid_from IS NULL OR valid_from<=CURRENT_TIMESTAMP) AND (valid_until IS NULL OR valid_until>CURRENT_TIMESTAMP) AND capability_code=? AND user_id = ANY(?) AND ((scope_type='global' AND scope_key='') OR (scope_type=? AND scope_key=?))`
	if err = c.service.database.GetScan(ctx, &rows, query, input.CapabilityCode, input.UserIDs, input.ScopeType, input.ScopeKey); err != nil {
		return nil, gerror.Wrap(err, "query staff business access")
	}
	for _, row := range rows {
		eligible[row.UserID] = true
	}
	result := make([]staffaccesscap.Decision, 0, len(input.UserIDs))
	for _, id := range input.UserIDs {
		decision := staffaccesscap.Decision{UserID: id, Reason: "qualification_missing"}
		if !active[id] {
			decision.Reason = "user_unavailable"
		} else {
			admin, adminErr := c.service.authorization.IsPlatformAdmin(ctx, authz.UserID(strconv.FormatInt(id, 10)))
			if adminErr != nil {
				return nil, gerror.Wrap(adminErr, "check target platform administrator")
			}
			decision.Eligible = admin || eligible[id]
			if decision.Eligible {
				decision.Reason = "eligible"
			}
		}
		result = append(result, decision)
	}
	return result, nil
}

func (s *Service) List(ctx context.Context, keyword, capabilityCode string, status, page, pageSize int) (*GrantPage, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	args := []any{}
	where := "g.tenant_id=0"
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		where += " AND (g.grant_no ILIKE ? OR CAST(g.user_id AS TEXT) ILIKE ? OR g.capability_code ILIKE ? OR g.scope_key ILIKE ?)"
		like := "%" + keyword + "%"
		args = append(args, like, like, like, like)
	}
	if capabilityCode = strings.TrimSpace(capabilityCode); capabilityCode != "" {
		where += " AND g.capability_code=?"
		args = append(args, capabilityCode)
	}
	if status == 0 || status == 1 {
		where += " AND g.status=?"
		args = append(args, status)
	}
	var count grantCountRow
	if err := s.database.GetScan(ctx, &count, "SELECT COUNT(*) AS total FROM plugin_staff_access_grant g WHERE "+where, args...); err != nil {
		return nil, err
	}
	rows := []Grant{}
	query := `SELECT g.grant_no,g.user_id,g.capability_code,g.scope_type,g.scope_key,g.status,g.valid_from,g.valid_until,g.version,g.created_at,g.updated_at FROM plugin_staff_access_grant g WHERE ` + where + ` ORDER BY g.updated_at DESC,g.id DESC LIMIT ? OFFSET ?`
	args = append(args, pageSize, (page-1)*pageSize)
	if err := s.database.GetScan(ctx, &rows, query, args...); err != nil {
		return nil, err
	}
	ids := make([]usercap.UserID, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, usercap.UserID(strconv.FormatInt(row.UserID, 10)))
	}
	if len(ids) > 0 {
		users, err := s.users.BatchGet(ctx, ids)
		if err != nil {
			return nil, err
		}
		byID := map[int64]*usercap.UserInfo{}
		for _, item := range users.Items {
			id, _ := strconv.ParseInt(string(item.ID), 10, 64)
			byID[id] = item
		}
		for i := range rows {
			if item := byID[rows[i].UserID]; item != nil {
				rows[i].Username = item.Username
				rows[i].Nickname = item.Nickname
			}
		}
	}
	return &GrantPage{Items: rows, Total: count.Total}, nil
}

func (s *Service) Candidates(ctx context.Context, keyword string, page, pageSize int) ([]UserCandidate, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	result, err := s.users.List(ctx, usercap.ListInput{Keyword: strings.TrimSpace(keyword), EnabledOnly: true, Page: capmodel.PageRequest{PageNum: page, PageSize: pageSize}})
	if err != nil {
		return nil, 0, gerror.Wrap(err, "list visible employee candidates")
	}
	items := make([]UserCandidate, 0, len(result.Items))
	for _, item := range result.Items {
		id, parseErr := strconv.ParseInt(string(item.ID), 10, 64)
		if parseErr == nil {
			items = append(items, UserCandidate{UserID: id, Username: item.Username, Nickname: item.Nickname})
		}
	}
	return items, result.Total, nil
}

func (s *Service) Save(ctx context.Context, input SaveInput) (*Grant, error) {
	if err := staffaccesscap.Validate(input.UserID, input.CapabilityCode, input.ScopeType, input.ScopeKey); err != nil {
		return nil, err
	}
	if input.ExpectedVersion < 0 {
		return nil, gerror.New("staff access version is invalid")
	}
	if input.Reason = strings.TrimSpace(input.Reason); len(input.Reason) < 3 || len(input.Reason) > 500 {
		return nil, gerror.New("change reason must contain 3 to 500 characters")
	}
	if input.ValidFrom != nil && input.ValidUntil != nil && !input.ValidUntil.After(*input.ValidFrom) {
		return nil, gerror.New("staff access expiry must be after its activation time")
	}
	actor, err := s.requireManager(ctx)
	if err != nil {
		return nil, err
	}
	if err = s.users.EnsureVisible(ctx, []usercap.UserID{usercap.UserID(strconv.FormatInt(input.UserID, 10))}); err != nil {
		return nil, gerror.New("target employee is unavailable")
	}
	grant := &Grant{}
	err = s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		status := 0
		if input.Enabled {
			status = 1
		}
		grantNo, eventNo, idErr := newNo("SAG"), newNo("SAE"), error(nil)
		if grantNo == "" || eventNo == "" {
			idErr = gerror.New("generate staff access evidence number")
		}
		if idErr != nil {
			return idErr
		}
		action := "grant"
		if !input.Enabled {
			action = "disable"
		}
		query := `INSERT INTO plugin_staff_access_grant (tenant_id,grant_no,user_id,capability_code,scope_type,scope_key,status,valid_from,valid_until,version,created_by,updated_by) SELECT 0,?,?,?,?,?,?,?,?,1,?,? WHERE ?=0 ON CONFLICT (tenant_id,user_id,capability_code,scope_type,scope_key) DO UPDATE SET status=EXCLUDED.status,valid_from=EXCLUDED.valid_from,valid_until=EXCLUDED.valid_until,version=plugin_staff_access_grant.version+1,updated_by=EXCLUDED.updated_by,updated_at=CURRENT_TIMESTAMP WHERE plugin_staff_access_grant.version=? RETURNING grant_no,user_id,capability_code,scope_type,scope_key,status,valid_from,valid_until,version,created_at,updated_at`
		if err := tx.GetScan(grant, query, grantNo, input.UserID, input.CapabilityCode, input.ScopeType, input.ScopeKey, status, input.ValidFrom, input.ValidUntil, actor, actor, input.ExpectedVersion, input.ExpectedVersion); err != nil {
			return err
		}
		if grant.GrantNo == "" {
			return gerror.New("staff access version changed; refresh and try again")
		}
		if grant.Version > 1 {
			if input.Enabled {
				action = "enable"
			} else {
				action = "disable"
			}
		}
		_, err := tx.Exec(`INSERT INTO plugin_staff_access_event (event_no,grant_no,action_code,actor_user_id,user_id,capability_code,scope_type,scope_key,reason,grant_version,valid_from,valid_until) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`, eventNo, grant.GrantNo, action, actor, input.UserID, input.CapabilityCode, input.ScopeType, input.ScopeKey, input.Reason, grant.Version, input.ValidFrom, input.ValidUntil)
		return err
	})
	if err != nil {
		return nil, gerror.Wrap(err, "save staff business access")
	}
	return grant, nil
}

func (s *Service) requireManager(ctx context.Context) (int64, error) {
	current := s.business.Current(ctx)
	if current.UserID <= 0 || current.IsImpersonation {
		return 0, gerror.New("staff access changes require a direct platform session")
	}
	allowed := current.IsSuperAdmin
	if !allowed {
		granted, err := s.authorization.HasPermission(ctx, authz.PermissionKey("system:staff-access:manage"))
		if err != nil {
			return 0, err
		}
		allowed = granted
	}
	if !allowed {
		return 0, gerror.New("staff access management permission is required")
	}
	session, err := s.sessions.Current(ctx)
	if err != nil || session == nil || session.LoginAt == nil || session.UserID != strconv.Itoa(current.UserID) {
		return 0, gerror.New("recent full login is required")
	}
	now := s.now().UTC()
	loginAt := session.LoginAt.UTC()
	if loginAt.After(now.Add(time.Minute)) || now.Sub(loginAt) > recentLoginWindow {
		return 0, gerror.New("recent full login is required")
	}
	return int64(current.UserID), nil
}

func newNo(prefix string) string {
	data := make([]byte, 12)
	if _, err := rand.Read(data); err != nil {
		return ""
	}
	return prefix + "_" + hex.EncodeToString(data)
}
