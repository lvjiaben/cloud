package admin

import (
	"context"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/operations"
	"time"
)

func (c *ControllerV1) OperationsWorkbenchGet(ctx context.Context, req *v1.OperationsWorkbenchGetReq) (*v1.OperationsWorkbenchGetRes, error) {
	if c.operations == nil {
		return nil, gerror.New("operations service is unavailable")
	}
	var since time.Time
	if req != nil && req.Since > 0 {
		since = time.UnixMilli(req.Since).UTC()
	}
	result, err := c.operations.Refresh(ctx, operations.RefreshRequest{Since: since})
	if err != nil {
		return nil, err
	}
	return operationsSnapshotToAPI(result.Snapshot, result.Changed), nil
}

func (c *ControllerV1) OperationsReportGet(ctx context.Context, req *v1.OperationsReportGetReq) (*v1.OperationsReportGetRes, error) {
	if c.operations == nil {
		return nil, gerror.New("operations service is unavailable")
	}
	query := operations.ReportQuery{}
	if req != nil {
		if req.From > 0 {
			query.From = time.UnixMilli(req.From).UTC()
		}
		if req.To > 0 {
			query.To = time.UnixMilli(req.To).UTC()
		}
		query.CurrencyCode = req.CurrencyCode
	}
	report, err := c.operations.LoadReport(ctx, query)
	if err != nil {
		return nil, err
	}
	res := &v1.OperationsReportGetRes{From: report.From.UnixMilli(), To: report.To.UnixMilli(), Currencies: make([]*v1.OperationsCurrencyMetric, 0, len(report.Currencies))}
	for _, metric := range report.Currencies {
		value := metric
		res.Currencies = append(res.Currencies, operationsMetricToAPI(value))
	}
	return res, nil
}

func (c *ControllerV1) OperationsTaskList(ctx context.Context, req *v1.OperationsTaskListReq) (*v1.OperationsTaskListRes, error) {
	if c.operations == nil {
		return nil, gerror.New("operations service is unavailable")
	}
	query := operations.TaskQuery{}
	if req != nil {
		query.Status, query.Limit = req.Status, req.Limit
	}
	tasks, err := c.operations.ListTasks(ctx, query)
	if err != nil {
		return nil, err
	}
	res := &v1.OperationsTaskListRes{Tasks: make([]*v1.OperationsTaskItem, 0, len(tasks))}
	for _, task := range tasks {
		res.Tasks = append(res.Tasks, &v1.OperationsTaskItem{TaskNo: task.TaskNo, TaskType: task.TaskType, SubjectType: task.SubjectType, SubjectNo: task.SubjectNo, Status: task.Status, ErrorCode: task.ErrorCode, ErrorSummary: task.ErrorSummary, AttemptCount: task.AttemptCount, CreatedAt: task.CreatedAt.UnixMilli(), UpdatedAt: task.UpdatedAt.UnixMilli()})
	}
	return res, nil
}

func (c *ControllerV1) OperationsTaskAlertsGet(ctx context.Context, _ *v1.OperationsTaskAlertsGetReq) (*v1.OperationsTaskAlertsGetRes, error) {
	if c.operations == nil {
		return nil, gerror.New("operations service is unavailable")
	}
	alerts, err := c.operations.ListTaskAlerts(ctx)
	if err != nil {
		return nil, err
	}
	res := &v1.OperationsTaskAlertsGetRes{Alerts: make([]*v1.OperationsTaskAlert, 0, len(alerts))}
	for _, alert := range alerts {
		res.Alerts = append(res.Alerts, &v1.OperationsTaskAlert{Code: alert.Code, Severity: alert.Severity, Count: alert.Count, OldestAt: alert.OldestAt.UnixMilli()})
	}
	return res, nil
}

func operationsSnapshotToAPI(snapshot operations.Snapshot, changed bool) *v1.OperationsWorkbenchGetRes {
	res := &v1.OperationsWorkbenchGetRes{GeneratedAt: snapshot.GeneratedAt.UnixMilli(), Watermark: snapshot.Watermark.UnixMilli(), Changed: changed, Queues: make([]*v1.OperationsQueueCount, 0, len(snapshot.Queues)), Currencies: make([]*v1.OperationsCurrencyMetric, 0, len(snapshot.Currencies))}
	for _, queue := range snapshot.Queues {
		value := queue
		res.Queues = append(res.Queues, &v1.OperationsQueueCount{Code: value.Code, Count: value.Count})
	}
	for _, metric := range snapshot.Currencies {
		value := metric
		res.Currencies = append(res.Currencies, operationsMetricToAPI(value))
	}
	return res
}

func operationsMetricToAPI(value operations.CurrencyMetric) *v1.OperationsCurrencyMetric {
	return &v1.OperationsCurrencyMetric{CurrencyCode: value.CurrencyCode, InvoiceCount: value.InvoiceCount, TotalAmountMinor: value.TotalAmountMinor, PaidAmountMinor: value.PaidAmountMinor, RefundedAmountMinor: value.RefundedAmountMinor, RevenueMinor: value.RevenueMinor, ReceivableMinor: value.ReceivableMinor, CashFlowMinor: value.CashFlowMinor, BalanceMinor: value.BalanceMinor, CostMinor: value.CostMinor, CostKnown: value.CostKnown, GrossProfitMinor: value.GrossProfitMinor, CustomerCount: value.CustomerCount, OrderCount: value.OrderCount, ProductCount: value.ProductCount, SalesCount: value.SalesCount, AffiliateCount: value.AffiliateCount, SupplierSuccess: value.SupplierSuccess, SupplierFailure: value.SupplierFailure}
}
