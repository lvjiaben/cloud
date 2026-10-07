package resourcetask

import (
	"context"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

const taskEvidenceKindResidualResource = "residualResource"

// AppendResidualResourceEvidence converts validated structured residual facts
// into owner-generated immutable evidence without persisting raw Provider data.
func AppendResidualResourceEvidence(evidence []resourcecap.Evidence,
	resources []resourcecap.ResidualResource, observedAt time.Time,
) ([]resourcecap.Evidence, error) {
	if len(resources) == 0 {
		return evidence, nil
	}
	if len(evidence)+len(resources) > resourcecap.MaximumEvidenceCount || observedAt.IsZero() {
		return nil, gerror.New("resource task residual evidence is invalid")
	}
	result := append([]resourcecap.Evidence(nil), evidence...)
	for _, item := range resources {
		if item.Validate() != nil {
			return nil, gerror.New("resource task residual resource is invalid")
		}
		result = append(result, resourcecap.Evidence{
			Kind: taskEvidenceKindResidualResource, Reference: item.ResourceRef,
			Summary:    item.ResourceKind + "|" + string(item.Status) + "|" + item.ReasonCode,
			ObservedAt: observedAt.UTC(),
		})
	}
	return result, nil
}

// LoadResidualResources returns at most the contract maximum from immutable
// owner evidence. Malformed rows fail closed instead of reaching core or UI.
func LoadResidualResources(ctx context.Context, database gdb.DB,
	taskNo string,
) ([]resourcecap.ResidualResource, error) {
	if database == nil || ctx == nil || ctx.Err() != nil || taskNo == "" {
		return nil, gerror.New("resource task residual resource query is invalid")
	}
	var rows []struct {
		Reference string `orm:"reference"`
		Summary   string `orm:"summary"`
	}
	if err := database.GetScan(ctx, &rows, `
SELECT reference,summary
FROM plugin_idc_resource_task_evidence
WHERE tenant_id=? AND task_no=? AND evidence_kind=?
ORDER BY observed_at ASC,id ASC
LIMIT ?`, idccap.DefaultTenantID, taskNo, taskEvidenceKindResidualResource,
		resourcecap.MaximumResidualResourceCount+1); err != nil {
		return nil, gerror.Wrap(err, "read resource task residual resources")
	}
	if len(rows) > resourcecap.MaximumResidualResourceCount {
		return nil, gerror.New("resource task residual resources exceed the contract limit")
	}
	result := make([]resourcecap.ResidualResource, len(rows))
	for index := range rows {
		parts := strings.Split(rows[index].Summary, "|")
		if len(parts) != 3 {
			return nil, gerror.New("resource task residual resource evidence is malformed")
		}
		result[index] = resourcecap.ResidualResource{ResourceKind: parts[0],
			ResourceRef: rows[index].Reference,
			Status:      resourcecap.ResidualResourceStatus(parts[1]), ReasonCode: parts[2]}
		if result[index].Validate() != nil {
			return nil, gerror.New("resource task residual resource evidence is invalid")
		}
	}
	return result, nil
}
