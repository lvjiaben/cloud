// This file owns staff-facing providerCapacity synchronization commands and bounded status reads.
package providercapacity

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/dao"
)

const (
	PermissionProviderCapacityList    = "idc:admin:resource:provider-capacity:list"
	PermissionProviderCapacityRefresh = "idc:admin:resource:provider-capacity:refresh"

	DefaultOperationPageSize = 25
	MaximumOperationPageSize = 100
	MaximumChannelOptions    = 500
)

var (
	CodeProviderCapacityInvalid = bizerr.MustDefine(
		"IDC_RESOURCE_PROVIDER_CAPACITY_INVALID", "The Provider capacity observation request is invalid", gcode.CodeInvalidParameter,
	)
	CodeProviderCapacityNotAccessible = bizerr.MustDefine(
		"IDC_RESOURCE_PROVIDER_CAPACITY_NOT_ACCESSIBLE", "The Provider capacity observation target is not accessible", gcode.CodeNotAuthorized,
	)
	CodeProviderCapacityConflict = bizerr.MustDefine(
		"IDC_RESOURCE_PROVIDER_CAPACITY_CONFLICT", "The Provider capacity observation conflicts with the current state", gcode.CodeInvalidOperation,
	)
	CodeProviderCapacityUnavailable = bizerr.MustDefine(
		"IDC_RESOURCE_PROVIDER_CAPACITY_UNAVAILABLE", "The Provider capacity observation service is temporarily unavailable", gcode.CodeInternalError,
	)
)

type OperationListInput struct {
	ChannelNo string
	Kind      resourcecap.ProviderCapacityKind
	Status    Status
	Cursor    string
	Limit     int
	Page      int
}

type Operation struct {
	ObservationNo  string
	ChannelNo      string
	ChannelName    string
	Kind           resourcecap.ProviderCapacityKind
	Status         Status
	Trigger        string
	ItemCount      int
	PageCount      int
	FailureCode    string
	RequestedAt    time.Time
	LastObservedAt time.Time
	CompletedAt    *time.Time
}

type OperationPage struct {
	Items      []Operation
	Total      int
	NextCursor string
	Channels   []ChannelOption
}

type ChannelOption struct {
	ChannelNo   string
	DisplayName string
}

type ManagementService interface {
	ListOperations(ctx context.Context, input OperationListInput) (*OperationPage, error)
	RequestSynchronization(
		ctx context.Context,
		channelNo string,
		kind resourcecap.ProviderCapacityKind,
	) (*Observation, error)
}

type managementService struct {
	database    gdb.DB
	runtime     managementcap.Runtime
	observation *serviceImpl
}

func NewManagement(database gdb.DB, runtime managementcap.Runtime) (ManagementService, error) {
	if database == nil || managementcap.ValidateRuntime(runtime) != nil {
		return nil, gerror.New("resource providerCapacity management dependencies are invalid")
	}
	return &managementService{
		database:    database,
		runtime:     runtime,
		observation: &serviceImpl{database: database, random: rand.Reader},
	}, nil
}

func (s *managementService) RequestSynchronization(
	ctx context.Context,
	channelNo string,
	kind resourcecap.ProviderCapacityKind,
) (*Observation, error) {
	actorUserID, err := s.authorize(ctx, PermissionProviderCapacityRefresh, channelNo)
	if err != nil {
		return nil, err
	}
	input := BeginInput{ChannelNo: strings.TrimSpace(channelNo), Kind: kind, PageSize: AutomaticPageSize}
	if err = validateBeginInput(input); err != nil {
		return nil, bizerr.NewCode(CodeProviderCapacityInvalid)
	}
	created, err := s.observation.begin(ctx, input, beginOptions{
		TriggerCode: TriggerManual, RequestedBy: actorUserID,
	})
	if err != nil {
		if errors.Is(err, ErrObservationAlreadyCollecting) {
			return nil, bizerr.NewCode(CodeProviderCapacityConflict)
		}
		return nil, bizerr.WrapCode(err, CodeProviderCapacityUnavailable)
	}
	return &created, nil
}

func (s *managementService) ListOperations(
	ctx context.Context,
	input OperationListInput,
) (*OperationPage, error) {
	if _, err := s.authorize(ctx, PermissionProviderCapacityList, "providerCapacity-observation-list"); err != nil {
		return nil, err
	}
	normalized, err := normalizeOperationList(input)
	if err != nil {
		return nil, err
	}
	model := s.operationListModel(ctx, normalized)
	total, err := model.Clone().Count()
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeProviderCapacityUnavailable)
	}
	if normalized.Page > 0 {
		model = model.Offset((normalized.Page - 1) * normalized.Limit)
	}
	if normalized.Cursor != "" {
		var boundary struct {
			ID        int64      `json:"id"`
			CreatedAt *time.Time `json:"createdAt"`
		}
		err = s.operationListModel(ctx, normalized).
			Fields("s.id", "s.created_at").
			Where("s.observation_no", normalized.Cursor).
			Limit(1).Scan(&boundary)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, bizerr.WrapCode(err, CodeProviderCapacityUnavailable)
		}
		if boundary.ID == 0 || boundary.CreatedAt == nil {
			return nil, bizerr.NewCode(CodeProviderCapacityInvalid)
		}
		model = model.Where(
			"(s.created_at < ?) OR (s.created_at = ? AND s.id < ?)",
			*boundary.CreatedAt, *boundary.CreatedAt, boundary.ID,
		)
	}
	type operationRow struct {
		ObservationNo string     `json:"observationNo"`
		ChannelNo     string     `json:"channelNo"`
		ChannelName   string     `json:"channelName"`
		CapacityKind  string     `orm:"capacity_kind"`
		StatusCode    string     `json:"statusCode"`
		TriggerCode   string     `json:"triggerCode"`
		ItemCount     int        `json:"itemCount"`
		PageCount     int        `json:"pageCount"`
		FailureCode   string     `json:"failureCode"`
		CreatedAt     *time.Time `json:"createdAt"`
		ObservedAt    *time.Time `json:"observedAt"`
		CompletedAt   *time.Time `json:"completedAt"`
	}
	rows := make([]operationRow, 0, normalized.Limit+1)
	err = model.Fields(
		"s.observation_no", "s.channel_no", "c.display_name AS channel_name",
		"s.capacity_kind", "s.status_code", "s.trigger_code", "s.item_count",
		"s.page_count", "s.failure_code", "s.created_at", "s.observed_at", "s.completed_at",
	).OrderDesc("s.created_at").OrderDesc("s.id").Limit(normalized.Limit + 1).Scan(&rows)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeProviderCapacityUnavailable)
	}
	hasMore := len(rows) > normalized.Limit
	if hasMore {
		rows = rows[:normalized.Limit]
	}
	items := make([]Operation, 0, len(rows))
	for _, row := range rows {
		if row.CreatedAt == nil || row.ObservedAt == nil {
			return nil, bizerr.NewCode(CodeProviderCapacityUnavailable)
		}
		items = append(items, Operation{
			ObservationNo: row.ObservationNo, ChannelNo: row.ChannelNo, ChannelName: row.ChannelName,
			Kind: resourcecap.ProviderCapacityKind(row.CapacityKind), Status: Status(row.StatusCode),
			Trigger: row.TriggerCode, ItemCount: row.ItemCount, PageCount: row.PageCount,
			FailureCode: row.FailureCode, RequestedAt: row.CreatedAt.UTC(),
			LastObservedAt: row.ObservedAt.UTC(), CompletedAt: utcTimePointer(row.CompletedAt),
		})
	}
	channels, err := s.listEnabledChannelOptions(ctx)
	if err != nil {
		return nil, err
	}
	page := &OperationPage{Items: items, Channels: channels, Total: total}
	if hasMore && len(items) > 0 {
		page.NextCursor = items[len(items)-1].ObservationNo
	}
	return page, nil
}

func (s *managementService) listEnabledChannelOptions(ctx context.Context) ([]ChannelOption, error) {
	columns := dao.Channel.Columns()
	type channelRow struct {
		ChannelNo   string `json:"channelNo"`
		DisplayName string `json:"displayName"`
	}
	rows := make([]channelRow, 0, MaximumChannelOptions+1)
	err := s.database.Model(dao.Channel.Table()).Safe().Ctx(ctx).
		Fields(columns.ChannelNo, columns.DisplayName).
		Where(columns.TenantId, idccap.DefaultTenantID).
		Where(columns.StatusCode, "enabled").WhereNull(columns.DeletedAt).
		OrderAsc(columns.DisplayName).OrderAsc(columns.ChannelNo).
		Limit(MaximumChannelOptions + 1).Scan(&rows)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeProviderCapacityUnavailable)
	}
	if len(rows) > MaximumChannelOptions {
		return nil, bizerr.NewCode(CodeProviderCapacityUnavailable)
	}
	result := make([]ChannelOption, len(rows))
	for index, row := range rows {
		result[index] = ChannelOption{ChannelNo: row.ChannelNo, DisplayName: row.DisplayName}
	}
	return result, nil
}

func (s *managementService) authorize(
	ctx context.Context,
	permission string,
	targetNo string,
) (int, error) {
	if s == nil || ctx == nil || ctx.Err() != nil {
		return 0, bizerr.NewCode(CodeProviderCapacityNotAccessible)
	}
	actor, err := s.runtime.Authorizer.Authorize(ctx, managementcap.AuthorizationRequest{
		Permission: permission, TargetNo: strings.TrimSpace(targetNo),
	})
	if err != nil || actor <= 0 {
		return 0, bizerr.NewCode(CodeProviderCapacityNotAccessible)
	}
	return actor, nil
}

func (s *managementService) operationListModel(
	ctx context.Context,
	input OperationListInput,
) *gdb.Model {
	model := s.database.Model(dao.ProviderCapacityObservation.Table()+" s").Safe().Ctx(ctx).
		LeftJoin(dao.Channel.Table()+" c", "c.channel_no=s.channel_no AND c.tenant_id=s.tenant_id").
		Where("s.tenant_id", idccap.DefaultTenantID)
	if input.ChannelNo != "" {
		model = model.Where("s.channel_no", input.ChannelNo)
	}
	if input.Kind != "" {
		model = model.Where("s.capacity_kind", input.Kind)
	}
	if input.Status != "" {
		model = model.Where("s.status_code", input.Status)
	}
	return model
}

func normalizeOperationList(input OperationListInput) (OperationListInput, error) {
	input.ChannelNo = strings.TrimSpace(input.ChannelNo)
	input.Cursor = strings.TrimSpace(input.Cursor)
	if input.Limit == 0 {
		input.Limit = DefaultOperationPageSize
	}
	if input.Limit < 1 || input.Limit > MaximumOperationPageSize ||
		input.Page < 0 || input.Page > 1_000_000 || (input.Page > 0 && input.Cursor != "") ||
		(input.ChannelNo != "" && resourcecap.ValidateChannelRef(input.ChannelNo) != nil) ||
		(input.Cursor != "" && !observationNumberPattern.MatchString(input.Cursor)) {
		return OperationListInput{}, bizerr.NewCode(CodeProviderCapacityInvalid)
	}
	if input.Kind != "" {
		if err := validateBeginInput(BeginInput{
			ChannelNo: "validation", Kind: input.Kind, PageSize: AutomaticPageSize,
		}); err != nil {
			return OperationListInput{}, bizerr.NewCode(CodeProviderCapacityInvalid)
		}
	}
	switch input.Status {
	case "", StatusCollecting, StatusComplete, StatusFailed, StatusSuperseded:
	default:
		return OperationListInput{}, bizerr.NewCode(CodeProviderCapacityInvalid)
	}
	return input, nil
}

func utcTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	utc := value.UTC()
	return &utc
}
