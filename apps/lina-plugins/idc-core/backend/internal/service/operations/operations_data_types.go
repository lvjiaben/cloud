// This file defines operating-data projections shared by the read service.

package operations

// DataPeriod is the bounded operating-data dataperiod projection.
type DataPeriod struct {
	Key         string
	StartAt     int64
	EndAt       int64
	IncomeMinor string
	PaidOrders  int64
}

// DataTrend is the bounded operating-data datatrend projection.
type DataTrend struct {
	Date            string
	PreviousDate    string
	StartAt         int64
	EndAt           int64
	PreviousStartAt int64
	PreviousEndAt   int64
	CurrentMinor    string
	CurrentCount    int64
	PreviousCount   *int64
}

// DataProduct is the bounded operating-data dataproduct projection.
type DataProduct struct {
	ProductNo string
	NameZhCN  string
	NameEnUS  string
	Quantity  int64
}

// DataCustomers is the bounded operating-data datacustomers projection.
type DataCustomers struct {
	Today  int64
	Total  int64
	Online int64
}

// DataTickets is the bounded operating-data datatickets projection.
type DataTickets struct {
	Pending   int64
	Processed int64
}

// DataRecord is the bounded operating-data datarecord projection.
type DataRecord struct {
	ID           string
	Label        string
	CustomerNo   string
	CustomerName string
	CurrencyCode string
	AmountMinor  string
	Quantity     int64
	StatusCode   string
	OccurredAt   int64
}

// Data is the bounded operating-data data projection.
type Data struct {
	Currencies       []string
	CurrencyCode     string
	CurrencyExponent int
	Timezone         string
	AsOf             int64
	Periods          []DataPeriod
	Trend            []DataTrend
	Products         []DataProduct
	Customers        DataCustomers
	ExpiringServers  int64
	Tickets          DataTickets
}

// DataRecords is the bounded operating-data datarecords projection.
type DataRecords struct {
	Items []DataRecord
	Total int64
}
