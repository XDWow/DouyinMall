package common

import (
	"context"
	"strings"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	globalnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/global"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

// LookupReadCache is called inside read-only business subgraphs, after
// UnderstandingNode has produced intent/rewrite/slots but before the expensive
// RAG/tool/agent path runs.
func LookupReadCache(ctx context.Context, lookup *globalnode.CacheLookupNode, intent domain.Intent) (*domain.ChatResult, error) {
	if lookup == nil || !domain.DefaultReadOnlyForIntent(intent) {
		return nil, nil
	}
	st := domain.SharedGraphState(ctx)
	if st == nil || st.Input == nil {
		return nil, nil
	}
	sessionID := ""
	if st.Session != nil {
		sessionID = strings.TrimSpace(st.Session.SessionID)
	}

	out, err := lookup.Invoke(ctx, globalnode.CacheLookupInput{
		TenantID:       globalnode.TenantIDOf(st, ""),
		UserID:         st.Input.UserID,
		SessionID:      sessionID,
		TraceID:        strings.TrimSpace(st.TraceID),
		Intent:         intent,
		Query:          strings.TrimSpace(st.Input.Message),
		RewrittenQuery: strings.TrimSpace(support.FirstNonEmpty(st.RewrittenQuery, st.Input.Message)),
	})
	if err != nil || !out.Hit || out.Response == nil {
		return nil, err
	}
	return out.Response, nil
}
