package node

import (
	"context"
	"sort"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	graphstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

type RerankNodeDeps struct {
	TopK int
}

type RerankNode struct{ deps RerankNodeDeps }

func NewRerankNode(deps RerankNodeDeps) *RerankNode {
	return &RerankNode{deps: deps}
}

func (n *RerankNode) Invoke(ctx context.Context, state *graphstate.ConversationState) (*graphstate.ConversationState, error) {
	if state == nil || len(state.Retrieval.Documents) == 0 {
		graphstate.BindConversationState(ctx, state)
		return state, nil
	}

	refs := make([]domain.KnowledgeRef, 0, len(state.Retrieval.Documents))
	queryTokens := support.SplitTerms(support.FirstNonEmpty(state.Session.RewrittenQuery, state.Session.RawQuery))
	for _, doc := range state.Retrieval.Documents {
		if doc == nil {
			continue
		}
		ref := domain.KnowledgeRef{
			ID:       doc.ID,
			Title:    metaString(doc.MetaData, "title"),
			Snippet:  support.Summarize(doc.Content, 180),
			Category: metaString(doc.MetaData, "category"),
			Score:    doc.Score(),
			Metadata: metaStringMap(doc.MetaData),
		}
		if snippet := metaString(doc.MetaData, "snippet"); snippet != "" {
			ref.Snippet = snippet
		}
		overlap := support.KeywordOverlap(queryTokens, support.SplitTerms(ref.Title+" "+ref.Snippet))
		ref.Score = support.Clamp01(ref.Score*0.7 + overlap*0.3)
		refs = append(refs, ref)
	}

	sort.Slice(refs, func(i, j int) bool { return refs[i].Score > refs[j].Score })
	if n.deps.TopK > 0 && len(refs) > n.deps.TopK {
		refs = refs[:n.deps.TopK]
	}
	state.Retrieval.References = refs
	graphstate.BindConversationState(ctx, state)
	return state, nil
}

func metaString(meta map[string]any, key string) string {
	if meta == nil {
		return ""
	}
	value, _ := meta[key].(string)
	return value
}

func metaStringMap(meta map[string]any) map[string]string {
	if len(meta) == 0 {
		return nil
	}
	out := make(map[string]string, len(meta))
	for key, value := range meta {
		if str, ok := value.(string); ok {
			out[key] = str
		}
	}
	return out
}
