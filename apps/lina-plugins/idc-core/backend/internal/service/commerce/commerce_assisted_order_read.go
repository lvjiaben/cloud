package commerce

import (
	"context"
	"crypto/sha256"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"

	"lina-plugin-idc-core/backend/internal/service/customerprojection"
)

func (s *assistedQuoteServiceImpl) ListAssistedQuotes(
	ctx context.Context,
	filter AssistedQuoteFilter,
) (*AssistedQuotePage, error) {
	actor, err := s.authorizer.AuthorizeAssistedQuoteList(ctx)
	if err != nil {
		return nil, err
	}
	normalized, digest, boundary, err := s.normalizeAssistedQuoteFilter(filter)
	if err != nil {
		return nil, err
	}
	model := s.database.Model(dao.CommerceAssistedQuoteRequest.Table()).Ctx(ctx).
		Where(do.CommerceAssistedQuoteRequest{TenantId: idccap.DefaultTenantID})
	if normalized.CustomerNo != "" {
		model = model.Where(do.CommerceAssistedQuoteRequest{CustomerNo: normalized.CustomerNo})
	}
	if normalized.StatusCode != "" {
		model = model.Where(do.CommerceAssistedQuoteRequest{StatusCode: normalized.StatusCode})
	}
	var total int
	rows := make([]entity.CommerceAssistedQuoteRequest, 0, normalized.Limit+1)
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if _, readErr := s.database.Exec(ctx, "SET TRANSACTION ISOLATION LEVEL REPEATABLE READ, READ ONLY"); readErr != nil {
			return readErr
		}
		pageQuery := model.Clone().Ctx(ctx)
		var readErr error
		total, readErr = pageQuery.Clone().Count()
		if readErr != nil {
			return readErr
		}
		if boundary != nil {
			createdAt := time.UnixMicro(boundary.CreatedAtMicroseconds)
			pageQuery = pageQuery.Where("(created_at, id) < (?, ?)", createdAt, boundary.ID)
		}
		if normalized.Page > 0 {
			pageQuery = pageQuery.Offset((normalized.Page - 1) * normalized.Limit)
		}
		return pageQuery.OrderDesc(dao.CommerceAssistedQuoteRequest.Columns().CreatedAt).
			OrderDesc(dao.CommerceAssistedQuoteRequest.Columns().Id).
			Limit(normalized.Limit + 1).Scan(&rows)
	})
	if err != nil {
		return nil, bizerr.WrapCode(gerror.Wrap(err, "list assisted quote requests"), CodeAssistedQuoteUnavailable)
	}
	hasMore := len(rows) > normalized.Limit
	if hasMore {
		rows = rows[:normalized.Limit]
	}
	projections, err := loadAssistedQuoteProjections(ctx, s.database, rows, actor, false)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeAssistedQuoteUnavailable)
	}
	page := &AssistedQuotePage{Requests: projections, Total: total}
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		if last.CreatedAt == nil {
			return nil, bizerr.NewCode(CodeAssistedQuoteUnavailable)
		}
		page.NextCursor, err = s.cursor.encode(orderCursorBoundary{
			CreatedAtMicroseconds: last.CreatedAt.UnixMicro(), ID: last.Id,
		}, digest)
		if err != nil {
			return nil, bizerr.WrapCode(err, CodeAssistedQuoteUnavailable)
		}
	}
	return page, nil
}

func (s *assistedQuoteServiceImpl) GetAssistedQuote(
	ctx context.Context,
	requestNo string,
) (*AssistedQuoteRequest, error) {
	actor, err := s.authorizer.AuthorizeAssistedQuoteDetail(ctx, requestNo)
	if err != nil {
		return nil, err
	}
	request, err := loadAssistedQuoteRequest(ctx, s.database, requestNo, false)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeAssistedQuoteUnavailable)
	}
	if request == nil {
		return nil, bizerr.NewCode(CodeAssistedQuoteNotAccessible)
	}
	result, err := loadAssistedQuoteProjections(
		ctx, s.database, []entity.CommerceAssistedQuoteRequest{*request}, actor, false,
	)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeAssistedQuoteUnavailable)
	}
	return &result[0], nil
}

func (s *assistedQuoteServiceImpl) normalizeAssistedQuoteFilter(
	filter AssistedQuoteFilter,
) (AssistedQuoteFilter, [sha256.Size]byte, *orderCursorBoundary, error) {
	filter.CustomerNo = strings.TrimSpace(filter.CustomerNo)
	filter.Cursor = strings.TrimSpace(filter.Cursor)
	if filter.Limit == 0 {
		filter.Limit = defaultAssistedQuotePageLimit
	}
	if filter.Limit < 1 || filter.Limit > maximumAssistedQuotePageLimit ||
		filter.Page < 0 || (filter.Page > 0 && (filter.Cursor != "" || filter.Page-1 > math.MaxInt/filter.Limit)) ||
		(filter.CustomerNo != "" && !validAssistedIdentifier(filter.CustomerNo)) ||
		!validAssistedQuoteStatusFilter(filter.StatusCode) {
		return AssistedQuoteFilter{}, [sha256.Size]byte{}, nil,
			bizerr.NewCode(CodeAssistedQuoteInvalid)
	}
	digest := sha256.Sum256([]byte("assisted-quote-list/v1\x00" + filter.CustomerNo + "\x00" +
		string(filter.StatusCode) + "\x00" + strconv.Itoa(filter.Limit)))
	var boundary *orderCursorBoundary
	var err error
	if filter.Cursor != "" {
		boundary, err = s.cursor.decode(filter.Cursor, digest)
		if err != nil {
			return AssistedQuoteFilter{}, [sha256.Size]byte{}, nil,
				bizerr.NewCode(CodeAssistedQuoteInvalid)
		}
	}
	return filter, digest, boundary, nil
}

func validAssistedQuoteStatusFilter(status AssistedQuoteStatus) bool {
	switch status {
	case "", AssistedQuoteStatusPendingApproval, AssistedQuoteStatusApproved,
		AssistedQuoteStatusRejected, AssistedQuoteStatusOrdered:
		return true
	default:
		return false
	}
}

func loadAssistedQuoteRequest(
	ctx context.Context,
	database gdb.DB,
	requestNo string,
	lock bool,
) (*entity.CommerceAssistedQuoteRequest, error) {
	var result *entity.CommerceAssistedQuoteRequest
	model := database.Model(dao.CommerceAssistedQuoteRequest.Table()).Ctx(ctx).Where(
		do.CommerceAssistedQuoteRequest{TenantId: idccap.DefaultTenantID, RequestNo: requestNo},
	)
	if lock {
		model = model.LockUpdate()
	}
	if err := model.Scan(&result); err != nil {
		return nil, gerror.Wrap(err, "load assisted quote request")
	}
	return result, nil
}

func loadAssistedQuoteProjections(
	ctx context.Context,
	database gdb.DB,
	requests []entity.CommerceAssistedQuoteRequest,
	actor int,
	reused bool,
) ([]AssistedQuoteRequest, error) {
	if len(requests) == 0 {
		return []AssistedQuoteRequest{}, nil
	}
	requestNos := make([]string, 0, len(requests))
	customerNos := make([]string, 0, len(requests))
	quoteNos := make([]string, 0, len(requests))
	orderNos := make([]string, 0, len(requests))
	for _, request := range requests {
		requestNos = append(requestNos, request.RequestNo)
		customerNos = append(customerNos, request.CustomerNo)
		if request.QuoteNo != nil {
			quoteNos = append(quoteNos, *request.QuoteNo)
		}
		if request.OrderNo != nil {
			orderNos = append(orderNos, *request.OrderNo)
		}
	}
	customers, err := customerprojection.Load(ctx, database, customerNos)
	if err != nil {
		return nil, gerror.Wrap(err, "load assisted quote customers")
	}
	items := make([]entity.CommerceAssistedQuoteRequestItem, 0)
	if err := database.Model(dao.CommerceAssistedQuoteRequestItem.Table()).Ctx(ctx).
		Where(do.CommerceAssistedQuoteRequestItem{TenantId: idccap.DefaultTenantID}).
		WhereIn(dao.CommerceAssistedQuoteRequestItem.Columns().RequestNo, requestNos).
		OrderAsc(dao.CommerceAssistedQuoteRequestItem.Columns().RequestNo).
		OrderAsc(dao.CommerceAssistedQuoteRequestItem.Columns().LineNumber).
		Limit(len(requests)*maximumAssistedQuoteItems + 1).Scan(&items); err != nil {
		return nil, gerror.Wrap(err, "load assisted quote request items")
	}
	if len(items) > len(requests)*maximumAssistedQuoteItems {
		return nil, gerror.New("assisted quote request item bound exceeded")
	}
	type quantityRow struct {
		RequestItemNo string `orm:"request_item_no"`
		Quantity      int    `orm:"quantity"`
	}
	quantityRows := make([]quantityRow, 0, len(items))
	if err := database.GetScan(ctx, &quantityRows, `SELECT quantity.request_item_no,quantity.quantity FROM plugin_idc_core_commerce_assisted_quote_item_quantity quantity
JOIN plugin_idc_core_commerce_assisted_quote_request_item item ON item.tenant_id=quantity.tenant_id AND item.request_item_no=quantity.request_item_no
WHERE item.tenant_id=0 AND item.request_no IN(?) ORDER BY item.request_no,item.line_number,item.id LIMIT ?`, requestNos, len(requests)*maximumAssistedQuoteItems+1); err != nil || len(quantityRows) != len(items) {
		return nil, gerror.New("assisted quote generic quantity evidence is unavailable")
	}
	quantities := make(map[string]int, len(quantityRows))
	for _, row := range quantityRows {
		quantities[row.RequestItemNo] = row.Quantity
	}
	itemsByRequest := make(map[string][]AssistedQuoteItem, len(requests))
	for _, item := range items {
		itemsByRequest[item.RequestNo] = append(itemsByRequest[item.RequestNo], AssistedQuoteItem{
			RequestItemNo: item.RequestItemNo, LineNumber: item.LineNumber,
			ProductNameZhCN: item.ProductNameZhCn, ProductNameEnUS: item.ProductNameEnUs,
			Quantity: quantities[item.RequestItemNo], PurchaseCycles: item.PurchaseCycles,
		})
	}
	quotes := make(map[string]entity.CommerceQuoteSnapshot, len(quoteNos))
	if len(quoteNos) > 0 {
		var rows []entity.CommerceQuoteSnapshot
		if err := database.Model(dao.CommerceQuoteSnapshot.Table()).Ctx(ctx).
			Where(do.CommerceQuoteSnapshot{TenantId: idccap.DefaultTenantID}).
			WhereIn(dao.CommerceQuoteSnapshot.Columns().QuoteNo, quoteNos).Scan(&rows); err != nil {
			return nil, gerror.Wrap(err, "load assisted quote snapshots")
		}
		for _, row := range rows {
			quotes[row.QuoteNo] = row
		}
	}
	orders := make(map[string]entity.CommerceOrder, len(orderNos))
	if len(orderNos) > 0 {
		var rows []entity.CommerceOrder
		if err := database.Model(dao.CommerceOrder.Table()).Ctx(ctx).
			Where(do.CommerceOrder{TenantId: idccap.DefaultTenantID}).
			WhereIn(dao.CommerceOrder.Columns().OrderNo, orderNos).Scan(&rows); err != nil {
			return nil, gerror.Wrap(err, "load assisted quote orders")
		}
		for _, row := range rows {
			orders[row.OrderNo] = row
		}
	}
	now, err := quoteDatabaseNow(ctx, database)
	if err != nil {
		return nil, err
	}
	result := make([]AssistedQuoteRequest, 0, len(requests))
	for _, request := range requests {
		view, projectionErr := projectAssistedQuoteRequest(
			request, itemsByRequest[request.RequestNo], quotes, orders, now, actor, reused,
		)
		if projectionErr != nil {
			return nil, projectionErr
		}
		customer := customers[request.CustomerNo]
		view.CustomerCode = customer.CustomerCode
		view.CustomerDisplayName = customer.DisplayName
		view.CustomerPrimaryPhone = customer.PrimaryPhone
		result = append(result, view)
	}
	return result, nil
}

func projectAssistedQuoteRequest(
	request entity.CommerceAssistedQuoteRequest,
	items []AssistedQuoteItem,
	quotes map[string]entity.CommerceQuoteSnapshot,
	orders map[string]entity.CommerceOrder,
	_ time.Time,
	_ int,
	reused bool,
) (AssistedQuoteRequest, error) {
	if request.ExpiresAt == nil || request.CreatedAt == nil || len(items) != request.ItemCount {
		return AssistedQuoteRequest{}, gerror.New("assisted quote request projection is inconsistent")
	}
	view := AssistedQuoteRequest{
		RequestNo: request.RequestNo, CustomerNo: request.CustomerNo,
		CurrencyCode: request.CurrencyCode, StatusCode: AssistedQuoteStatus(request.StatusCode),
		ManualDiscountMinor: request.ManualDiscountMinor,
		PaymentTermCode:     AssistedPaymentTerm(request.PaymentTermCode),
		PaymentDueSeconds:   request.PaymentDueSeconds,

		Reason:       request.Reason,
		InternalNote: request.InternalNote, RequestedBy: request.RequestedBy,
		ExpiresAt: *request.ExpiresAt, CreatedAt: *request.CreatedAt,
		Items: items, Reused: reused,
	}
	projectAssistedDecision(&view, request)
	if err := projectAssistedQuoteSnapshot(&view, request, quotes); err != nil {
		return AssistedQuoteRequest{}, err
	}
	if err := projectAssistedOrder(&view, request, orders); err != nil {
		return AssistedQuoteRequest{}, err
	}
	return view, nil
}

func projectAssistedDecision(view *AssistedQuoteRequest, request entity.CommerceAssistedQuoteRequest) {
	if request.DecisionCode != nil {
		view.DecisionCode = AssistedQuoteDecision(*request.DecisionCode)
	}
	if request.DecisionBy != nil {
		view.DecisionBy = *request.DecisionBy
	}
	if request.DecisionReason != nil {
		view.DecisionReason = *request.DecisionReason
	}
	if request.DecisionAt != nil {
		decisionAt := *request.DecisionAt
		view.DecisionAt = &decisionAt
	}
}

func projectAssistedQuoteSnapshot(
	view *AssistedQuoteRequest,
	request entity.CommerceAssistedQuoteRequest,
	quotes map[string]entity.CommerceQuoteSnapshot,
) error {
	if request.QuoteNo == nil {
		return nil
	}
	view.QuoteNo = *request.QuoteNo
	quote, ok := quotes[*request.QuoteNo]
	if !ok || quote.CustomerNo == nil || *quote.CustomerNo != request.CustomerNo ||
		quote.CurrencyCode != request.CurrencyCode || quote.SourceCode != "staff" {
		return gerror.New("assisted quote snapshot projection is inconsistent")
	}
	view.SubtotalMinor = quote.SubtotalMinor
	view.DiscountMinor = quote.DiscountMinor
	view.TotalAmountMinor = quote.TotalAmountMinor
	return nil
}

func projectAssistedOrder(
	view *AssistedQuoteRequest,
	request entity.CommerceAssistedQuoteRequest,
	orders map[string]entity.CommerceOrder,
) error {
	if request.OrderNo == nil {
		return nil
	}
	view.OrderNo = *request.OrderNo
	order, ok := orders[*request.OrderNo]
	if !ok || order.AssistedRequestNo == nil || *order.AssistedRequestNo != request.RequestNo {
		return gerror.New("assisted quote order projection is inconsistent")
	}
	view.OrderStatusCode = order.StatusCode
	return nil
}
