// Guest result validation accepts only explicit, bounded relationships between
// declared directory fields; unknown fields cannot cross the guest boundary.
package backend

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestGuestCandidateRelationshipsRemainWhitelisted(t *testing.T) {
	request := json.RawMessage(`{"schemaVersion":"v1","requests":[{"fieldKey":"line"}]}`)
	const result = `{"Fields":[{"FieldKey":"line","Items":[{"Value":"7","LabelZhCN":"线路七","LabelEnUS":"Line Seven"}]}],"Relationships":[{"ParentFieldKey":"region","ChildFieldKey":"line","Links":[{"ParentValue":"2","ChildValue":"7"}]}]}`
	if err := validateResourceCandidateResult(json.RawMessage(result), request); err != nil {
		t.Fatal("valid relation was rejected by the managed guest", err)
	}
	for _, invalid := range []string{
		strings.Replace(result, `"ChildValue":"7"`, `"ChildValue":"missing"`, 1),
		strings.Replace(result, `"ParentFieldKey":"region"`, `"ParentFieldKey":"password"`, 1),
	} {
		if err := validateResourceCandidateResult(json.RawMessage(invalid), request); err == nil {
			t.Fatal("invalid relation crossed the managed guest boundary")
		}
	}
}
