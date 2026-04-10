package orchestrator

import "strings"

func applyConfigDefaults(cfg Config) Config {
	def := DefaultConfig()
	if cfg.RateLimitPerMinute == 0 {
		cfg.RateLimitPerMinute = def.RateLimitPerMinute
	}
	if cfg.ConversationWindow == 0 {
		cfg.ConversationWindow = def.ConversationWindow
	}
	if cfg.ExactCacheTTL == 0 {
		cfg.ExactCacheTTL = def.ExactCacheTTL
	}
	if cfg.SemanticCacheTTL == 0 {
		cfg.SemanticCacheTTL = def.SemanticCacheTTL
	}
	if cfg.SemanticCacheScore == 0 {
		cfg.SemanticCacheScore = def.SemanticCacheScore
	}
	if cfg.SemanticCacheTopK == 0 {
		cfg.SemanticCacheTopK = def.SemanticCacheTopK
	}
	if cfg.RetrieveTopK == 0 {
		cfg.RetrieveTopK = def.RetrieveTopK
	}
	if cfg.RetrieveMinScore == 0 {
		cfg.RetrieveMinScore = def.RetrieveMinScore
	}
	if cfg.ConfidenceThreshold == 0 {
		cfg.ConfidenceThreshold = def.ConfidenceThreshold
	}
	if cfg.MaxAnswerTokens == 0 {
		cfg.MaxAnswerTokens = def.MaxAnswerTokens
	}
	if strings.TrimSpace(cfg.DefaultTenantID) == "" {
		cfg.DefaultTenantID = def.DefaultTenantID
	}
	if len(cfg.InterruptBeforeNodes) == 0 && len(def.InterruptBeforeNodes) > 0 {
		cfg.InterruptBeforeNodes = append([]string(nil), def.InterruptBeforeNodes...)
	}
	return cfg
}
