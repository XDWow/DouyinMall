package graph

import "strings"

func applyConfigDefaults(cfg Config) Config {
	def := DefaultConfig()
	if cfg.RateLimitPerMinute == 0 {
		cfg.RateLimitPerMinute = def.RateLimitPerMinute
	}
	if cfg.ConversationWindow == 0 {
		cfg.ConversationWindow = def.ConversationWindow
	}
	if cfg.L0CacheTTL == 0 {
		cfg.L0CacheTTL = def.L0CacheTTL
	}
	if cfg.RetrieveTopK == 0 {
		cfg.RetrieveTopK = def.RetrieveTopK
	}
	if cfg.RetrieveMinScore == 0 {
		cfg.RetrieveMinScore = def.RetrieveMinScore
	}
	if cfg.RerankTopK == 0 {
		cfg.RerankTopK = def.RerankTopK
	}
	if cfg.ToolParallelism == 0 {
		cfg.ToolParallelism = def.ToolParallelism
	}
	if cfg.ConfidenceThreshold == 0 {
		cfg.ConfidenceThreshold = def.ConfidenceThreshold
	}
	if cfg.SummaryTriggerTurns == 0 {
		cfg.SummaryTriggerTurns = def.SummaryTriggerTurns
	}
	if cfg.MaxAnswerTokens == 0 {
		cfg.MaxAnswerTokens = def.MaxAnswerTokens
	}
	if cfg.StreamBuffer == 0 {
		cfg.StreamBuffer = def.StreamBuffer
	}
	if strings.TrimSpace(cfg.DefaultTenantID) == "" {
		cfg.DefaultTenantID = def.DefaultTenantID
	}
	if strings.TrimSpace(cfg.KBVersion) == "" {
		cfg.KBVersion = def.KBVersion
	}
	if cfg.FeatureFlags == (FeatureFlags{}) {
		cfg.FeatureFlags = def.FeatureFlags
	}
	return cfg
}
