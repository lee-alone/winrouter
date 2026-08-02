package observability

import "strings"

func ParseRuleHits(coreLog string) []RuleHit {
	return []RuleHit{
		{Outbound: "a-direct", Count: countOutbound(coreLog, "domestic-direct") + countOutbound(coreLog, "a-direct")},
		{Outbound: "b-direct", Count: countOutbound(coreLog, "foreign-direct") + countOutbound(coreLog, "b-direct")},
	}
}

func countOutbound(coreLog, tag string) int {
	return strings.Count(coreLog, "outbound/direct["+tag+"]: outbound")
}
