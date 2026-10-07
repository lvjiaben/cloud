package channel

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	"lina-plugin-idc-core/backend/cap/channelcommercecap"
)

type ChannelInput struct {
	ChannelID   string
	CustomerNo  string
	DisplayName string
}

type ChannelSummary struct {
	ChannelID       string    `orm:"channel_id"`
	CustomerNo      string    `orm:"customer_no"`
	DisplayName     string    `orm:"display_name"`
	StatusCode      string    `orm:"status_code"`
	NewSalesEnabled bool      `orm:"new_sales_enabled"`
	CreatedAt       time.Time `orm:"created_at"`
	UpdatedAt       time.Time `orm:"updated_at"`
}

type PublicationInput struct {
	ChannelID        string
	ProductVersionID int64
	Status           string
	Stock            int
	Purchase         channelcommercecap.Purchase
}

type PublicationSummary struct {
	ChannelID             string `orm:"channel_id"`
	ProductVersionID      int64  `orm:"product_version_id"`
	Currency              string `orm:"currency"`
	PriceMinor            int64  `orm:"price_minor"`
	Stock                 int    `orm:"stock"`
	Status                string `orm:"status"`
	PublicSlug            string `orm:"public_slug"`
	ConfigurationRevision string `orm:"configuration_revision"`
	BillingTermToken      string `orm:"billing_term_token"`
	PurchaseCycles        int    `orm:"purchase_cycles"`
	Quantity              int    `orm:"quantity"`
}

type RequestAudit struct {
	ID          int64      `orm:"id"`
	ChannelID   string     `orm:"channel_id"`
	RequestID   string     `orm:"request_id"`
	Operation   string     `orm:"operation"`
	Status      string     `orm:"status"`
	ResultCode  string     `orm:"result_code"`
	CreatedAt   time.Time  `orm:"created_at"`
	CompletedAt *time.Time `orm:"completed_at"`
}

type ChannelHealth struct {
	ChannelID           string
	StatusCode          string
	NewSalesEnabled     bool
	ActiveTokenCount    int
	PublicationCount    int
	PendingWebhookCount int
	LastRequestAt       *time.Time
}

// ListChannels pages the same platform account collection as the staff list.
func (s *service) ListChannels(ctx context.Context, query PageQuery) (PageResult[ChannelSummary], error) {
	query, err := normalizePage(query)
	if err != nil {
		return PageResult[ChannelSummary]{}, err
	}
	model := s.db.Model("plugin_idc_channel_account").Ctx(ctx).
		Order("updated_at DESC,channel_id")
	if query.Keyword != "" {
		model = model.Where("strpos(lower(display_name),lower(?))>0 OR strpos(lower(channel_id),lower(?))>0", query.Keyword, query.Keyword)
	}
	return queryPage(model, query, func(page *gdb.Model) ([]ChannelSummary, error) {
		var rows []ChannelSummary
		err := page.Fields("channel_id,customer_no,display_name,status_code,new_sales_enabled,created_at,updated_at").Scan(&rows)
		return rows, err
	})
}

func (s *service) SaveChannel(ctx context.Context, input ChannelInput) (ChannelSummary, error) {
	input.ChannelID = strings.TrimSpace(input.ChannelID)
	input.CustomerNo = strings.TrimSpace(input.CustomerNo)
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	if input.ChannelID == "" || len(input.ChannelID) > 80 || input.CustomerNo == "" ||
		len(input.CustomerNo) > 40 || input.DisplayName == "" || len(input.DisplayName) > 160 {
		return ChannelSummary{}, gerror.New("channel input is invalid")
	}
	var existing *struct {
		CustomerNo string `orm:"customer_no"`
	}
	if err := s.db.Model("plugin_idc_channel_account").Ctx(ctx).Fields("customer_no").
		Where("channel_id=?", input.ChannelID).Scan(&existing); err != nil {
		return ChannelSummary{}, err
	}
	if existing != nil && existing.CustomerNo != input.CustomerNo {
		return ChannelSummary{}, gerror.New("channel customer binding cannot be changed")
	}
	actor := s.actorID(ctx)
	_, err := s.db.Exec(ctx, `INSERT INTO plugin_idc_channel_account(channel_id,customer_no,display_name,status_code,new_sales_enabled,created_by,updated_by) VALUES(?,?,?,'pending_approval',FALSE,?,?) ON CONFLICT(channel_id) DO UPDATE SET display_name=EXCLUDED.display_name,updated_by=EXCLUDED.updated_by,updated_at=CURRENT_TIMESTAMP`, input.ChannelID, input.CustomerNo, input.DisplayName, actor, actor)
	if err != nil {
		return ChannelSummary{}, err
	}
	return s.channelSummary(ctx, input.ChannelID)
}

func (s *service) SetChannelStatus(ctx context.Context, channelID, action string) (ChannelSummary, error) {
	channelID = strings.TrimSpace(channelID)
	action = strings.ToLower(strings.TrimSpace(action))
	account, err := s.loadChannel(ctx, channelID)
	if err != nil {
		return ChannelSummary{}, err
	}
	actor := s.actorID(ctx)
	var result sql.Result
	switch action {
	case "approve", "resume":
		if _, err = s.core.ValidatePayer(ctx, account.CustomerNo); err != nil {
			return ChannelSummary{}, err
		}
		result, err = s.db.Exec(ctx, `UPDATE plugin_idc_channel_account SET status_code='approved',new_sales_enabled=TRUE,approved_by=?,approved_at=COALESCE(approved_at,CURRENT_TIMESTAMP),paused_at=NULL,updated_by=?,updated_at=CURRENT_TIMESTAMP WHERE channel_id=? AND status_code IN ('pending_approval','paused','approved')`, actor, actor, channelID)
	case "pause":
		result, err = s.db.Exec(ctx, `UPDATE plugin_idc_channel_account SET status_code='paused',new_sales_enabled=FALSE,paused_at=CURRENT_TIMESTAMP,updated_by=?,updated_at=CURRENT_TIMESTAMP WHERE channel_id=? AND status_code='approved'`, actor, channelID)
	case "close":
		result, err = s.db.Exec(ctx, `UPDATE plugin_idc_channel_account SET status_code='closed',new_sales_enabled=FALSE,paused_at=COALESCE(paused_at,CURRENT_TIMESTAMP),updated_by=?,updated_at=CURRENT_TIMESTAMP WHERE channel_id=? AND status_code IN ('pending_approval','paused')`, actor, channelID)
	default:
		return ChannelSummary{}, gerror.New("channel status action is invalid")
	}
	if err != nil {
		return ChannelSummary{}, err
	}
	affected, affectedErr := result.RowsAffected()
	if affectedErr != nil || affected != 1 {
		return ChannelSummary{}, gerror.New("channel status transition is not allowed")
	}
	return s.channelSummary(ctx, channelID)
}

func (s *service) channelSummary(ctx context.Context, channelID string) (ChannelSummary, error) {
	var row ChannelSummary
	err := s.db.Model("plugin_idc_channel_account").Ctx(ctx).
		Fields("channel_id,customer_no,display_name,status_code,new_sales_enabled,created_at,updated_at").
		Where("channel_id=?", channelID).Scan(&row)
	if err != nil || row.ChannelID == "" {
		if err == nil {
			err = gerror.New("channel not found")
		}
		return ChannelSummary{}, err
	}
	return row, nil
}

// ListPublications keeps count and rows within the selected existing channel.
func (s *service) ListPublications(ctx context.Context, channelID string, query PageQuery) (PageResult[PublicationSummary], error) {
	if _, err := s.loadChannel(ctx, channelID); err != nil {
		return PageResult[PublicationSummary]{}, err
	}
	model := s.db.Model("plugin_idc_channel_publication").Ctx(ctx).
		Where("channel_id=?", strings.TrimSpace(channelID)).Order("id DESC")
	return queryPage(model, query, func(page *gdb.Model) ([]PublicationSummary, error) {
		var rows []PublicationSummary
		err := page.Fields("channel_id,product_version_id,currency,price_minor,stock,status,public_slug,configuration_revision,billing_term_token,purchase_cycles,quantity").Scan(&rows)
		return rows, err
	})
}

func (s *service) SavePublication(ctx context.Context, input PublicationInput) (PublicationSummary, error) {
	input.ChannelID = strings.TrimSpace(input.ChannelID)
	input.Status = strings.ToLower(strings.TrimSpace(input.Status))
	input.Purchase.CurrencyCode = strings.ToUpper(strings.TrimSpace(input.Purchase.CurrencyCode))
	if input.ProductVersionID <= 0 || input.Stock < 0 || (input.Status != "published" && input.Status != "paused") {
		return PublicationSummary{}, gerror.New("channel publication input is invalid")
	}
	account, err := s.requireChannel(ctx, input.ChannelID, input.Status == "published")
	if err != nil {
		return PublicationSummary{}, err
	}
	quote, err := s.core.Quote(ctx, channelcommercecap.QuoteInput{PayerCustomerNo: account.CustomerNo, Purchase: input.Purchase})
	if err != nil {
		return PublicationSummary{}, err
	}
	values, err := json.Marshal(input.Purchase.Values)
	if err != nil {
		return PublicationSummary{}, err
	}
	_, err = s.db.Exec(ctx, `INSERT INTO plugin_idc_channel_publication(channel_id,product_version_id,currency,price,stock,status,public_slug,configuration_revision,billing_term_token,purchase_cycles,quantity,purchase_values,price_minor) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(channel_id,product_version_id,currency) DO UPDATE SET price=EXCLUDED.price,stock=EXCLUDED.stock,status=EXCLUDED.status,public_slug=EXCLUDED.public_slug,configuration_revision=EXCLUDED.configuration_revision,billing_term_token=EXCLUDED.billing_term_token,purchase_cycles=EXCLUDED.purchase_cycles,quantity=EXCLUDED.quantity,purchase_values=EXCLUDED.purchase_values,price_minor=EXCLUDED.price_minor`, input.ChannelID, input.ProductVersionID, input.Purchase.CurrencyCode, strconv.FormatInt(quote.TotalAmountMinor, 10), input.Stock, input.Status, input.Purchase.PublicSlug, input.Purchase.ConfigurationRevision, input.Purchase.BillingTermToken, input.Purchase.PurchaseCycles, input.Purchase.Quantity, string(values), quote.TotalAmountMinor)
	if err != nil {
		return PublicationSummary{}, err
	}
	var row PublicationSummary
	err = s.db.Model("plugin_idc_channel_publication").Ctx(ctx).
		Fields("channel_id,product_version_id,currency,price_minor,stock,status,public_slug,configuration_revision,billing_term_token,purchase_cycles,quantity").
		Where("channel_id=? AND product_version_id=? AND currency=?", input.ChannelID, input.ProductVersionID, input.Purchase.CurrencyCode).Scan(&row)
	return row, err
}

// ListRequestAudit pages immutable requests without leaking other channels' totals.
func (s *service) ListRequestAudit(ctx context.Context, channelID string, query PageQuery) (PageResult[RequestAudit], error) {
	if _, err := s.loadChannel(ctx, channelID); err != nil {
		return PageResult[RequestAudit]{}, err
	}
	model := s.db.Model("plugin_idc_channel_request").Ctx(ctx).
		Where("channel_id=?", strings.TrimSpace(channelID)).Order("id DESC")
	return queryPage(model, query, func(page *gdb.Model) ([]RequestAudit, error) {
		var rows []RequestAudit
		err := page.Fields("id,channel_id,request_id,operation,status,result_code,created_at,completed_at").Scan(&rows)
		return rows, err
	})
}

func (s *service) Health(ctx context.Context, channelID string) (ChannelHealth, error) {
	account, err := s.loadChannel(ctx, channelID)
	if err != nil {
		return ChannelHealth{}, err
	}
	var row struct {
		ActiveTokenCount    int        `orm:"active_token_count"`
		PublicationCount    int        `orm:"publication_count"`
		PendingWebhookCount int        `orm:"pending_webhook_count"`
		LastRequestAt       *time.Time `orm:"last_request_at"`
	}
	err = s.db.GetScan(ctx, &row, `SELECT
 (SELECT COUNT(*) FROM plugin_idc_channel_token WHERE channel_id=? AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at>CURRENT_TIMESTAMP)) AS active_token_count,
 (SELECT COUNT(*) FROM plugin_idc_channel_publication WHERE channel_id=? AND status='published') AS publication_count,
 (SELECT COUNT(*) FROM plugin_idc_channel_webhook WHERE channel_id=? AND status IN ('pending','retry','delivering')) AS pending_webhook_count,
 (SELECT MAX(created_at) FROM plugin_idc_channel_request WHERE channel_id=?) AS last_request_at`, channelID, channelID, channelID, channelID)
	if err != nil {
		return ChannelHealth{}, err
	}
	return ChannelHealth{ChannelID: account.ChannelID, StatusCode: account.StatusCode, NewSalesEnabled: account.NewSalesEnable, ActiveTokenCount: row.ActiveTokenCount, PublicationCount: row.PublicationCount, PendingWebhookCount: row.PendingWebhookCount, LastRequestAt: row.LastRequestAt}, nil
}

func (s *service) currentApprovedChannel(ctx context.Context) (*channelAccountRow, error) {
	current, err := s.core.CurrentCustomer(ctx)
	if err != nil || current.CustomerTypeCode != "enterprise" || current.Status != "active" || current.IdentityStatus != "verified" {
		return nil, gerror.New("current customer is not an approved enterprise channel")
	}
	var row *channelAccountRow
	err = s.db.Model("plugin_idc_channel_account").Ctx(ctx).
		Fields("channel_id,customer_no,display_name,status_code,new_sales_enabled").
		Where("customer_no=? AND status_code='approved' AND new_sales_enabled=TRUE", current.CustomerNo).Scan(&row)
	if err != nil || row == nil {
		if err == nil {
			err = gerror.New("current customer is not an approved enterprise channel")
		}
		return nil, err
	}
	return row, nil
}

func (s *service) CreateOwnToken(ctx context.Context, input TokenInput) (TokenResult, error) {
	channel, err := s.currentApprovedChannel(ctx)
	if err != nil {
		return TokenResult{}, err
	}
	input.ChannelID = channel.ChannelID
	return s.CreateToken(ctx, input)
}

func (s *service) ListOwnTokens(ctx context.Context) ([]TokenSummary, error) {
	channel, err := s.currentApprovedChannel(ctx)
	if err != nil {
		return nil, err
	}
	return s.ListTokens(ctx, channel.ChannelID)
}

func (s *service) RevokeOwnToken(ctx context.Context, id int64) error {
	channel, err := s.currentApprovedChannel(ctx)
	if err != nil {
		return err
	}
	result, err := s.db.Model("plugin_idc_channel_token").Ctx(ctx).Data(g.Map{"revoked_at": time.Now()}).Where("id=? AND channel_id=? AND revoked_at IS NULL", id, channel.ChannelID).Update()
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil || count != 1 {
		return gerror.New("token not found or already revoked")
	}
	return nil
}
