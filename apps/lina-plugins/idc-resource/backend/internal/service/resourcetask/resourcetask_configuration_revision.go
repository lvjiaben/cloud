package resourcetask

import (
	"encoding/json"
	"regexp"
)

var taskConfigurationRevisionPattern = regexp.MustCompile(`^rci_[0-9a-f]{32}$`)

func taskConfigurationRevision(metadata string) string {
	var facts struct {
		Revision string `json:"interfaceConfigurationRevision"`
	}
	if json.Unmarshal([]byte(metadata), &facts) != nil ||
		!taskConfigurationRevisionPattern.MatchString(facts.Revision) {
		return ""
	}
	return facts.Revision
}
