package staffaccesscap

import (
	"regexp"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
)

var (
	capabilityPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{2,127}$`)
	scopeKeyPattern   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$`)
)

func Validate(userID int64, capabilityCode, scopeType, scopeKey string) error {
	if userID <= 0 || capabilityCode != strings.TrimSpace(capabilityCode) || !capabilityPattern.MatchString(capabilityCode) {
		return gerror.New("staff access target or capability is invalid")
	}
	if scopeType == ScopeGlobal && scopeKey == "" {
		return nil
	}
	if scopeType == ScopeDepartment && scopeKey == strings.TrimSpace(scopeKey) && scopeKeyPattern.MatchString(scopeKey) {
		return nil
	}
	return gerror.New("staff access scope is invalid")
}
