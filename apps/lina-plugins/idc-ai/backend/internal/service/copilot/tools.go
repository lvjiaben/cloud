package copilot

// Tool is intentionally a data-only descriptor. There is no Execute method:
// the AI plugin can only ask owners for bounded read snapshots.
type Tool struct {
	Name        string
	Owner       string
	Description string
}

func ReadOnlyTools() []Tool {
	return []Tool{
		{Name: "operations.summary", Owner: "idc-core", Description: "Read the bounded operations summary"},
		{Name: "billing.overview", Owner: "idc-core", Description: "Read billing totals visible to the actor"},
		{Name: "resource.status", Owner: "idc-resource", Description: "Read resource status and provider evidence"},
		{Name: "support.search", Owner: "idc-core", Description: "Search approved support knowledge"},
		{Name: "customer.services", Owner: "idc-core", Description: "Read services for the current customer only"},
	}
}
