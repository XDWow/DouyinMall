package domain

import "strings"

// EffectiveRewrittenQuery 路由级 RewrittenQuery。
func EffectiveRewrittenQuery(s *State) string {
	if s == nil {
		return ""
	}
	return strings.TrimSpace(s.RewrittenQuery)
}
