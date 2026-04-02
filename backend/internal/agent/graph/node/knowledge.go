package node

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/dto"
	orchestratorstate "github.com/XDWow/DouyinMall/backend/internal/agent/graph/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/graph/support"
)

type RewriteNode struct{ suite *Suite }
type RetrieveNode struct{ suite *Suite }
type RerankNode struct{ suite *Suite }
type FallbackNode struct{ suite *Suite }

func (s *Suite) Rewrite() *RewriteNode   { return &RewriteNode{suite: s} }
func (s *Suite) Retrieve() *RetrieveNode { return &RetrieveNode{suite: s} }
func (s *Suite) Rerank() *RerankNode     { return &RerankNode{suite: s} }
func (s *Suite) Fallback() *FallbackNode { return &FallbackNode{suite: s} }

func (n *RewriteNode) Evaluate(ctx context.Context, flow *orchestratorstate.FlowContext) (*orchestratorstate.FlowContext, error) {
	orchestratorstate.BindConversationFlow(ctx, flow)
	return flow, nil
}

func (n *RewriteNode) Identity(ctx context.Context, flow *orchestratorstate.FlowContext) (*orchestratorstate.FlowContext, error) {
	if flow == nil {
		return nil, fmt.Errorf("flow is required")
	}
	flow.Rewrite = orchestratorstate.RewriteDecision{Query: strings.TrimSpace(support.FirstNonEmpty(flow.State.RawQuery, flow.Request.Message)), Reason: "identity"}
	flow.State.RewrittenQuery = flow.Rewrite.Query
	orchestratorstate.BindConversationFlow(ctx, flow)
	return flow, nil
}

func (n *RewriteNode) BuildPromptInput(ctx context.Context, flow *orchestratorstate.FlowContext) (map[string]any, error) {
	orchestratorstate.BindConversationFlow(ctx, flow)
	if flow == nil {
		return nil, fmt.Errorf("flow is required")
	}
	return map[string]any{
		"system_text":  n.suite.deps.Prompts.SystemText,
		"history_text": support.HistoryText(flow.Session, 6),
		"message":      flow.State.RawQuery,
		"intent":       string(flow.State.Intent),
	}, nil
}

func (n *RewriteNode) Apply(ctx context.Context, msg *schema.Message) (*orchestratorstate.FlowContext, error) {
	flow := orchestratorstate.ConversationFlowFromContext(ctx)
	if flow == nil {
		return nil, fmt.Errorf("conversation flow is missing")
	}
	query := strings.TrimSpace(support.FirstNonEmpty(flow.State.RawQuery, flow.Request.Message))
	reason := "identity"
	if msg != nil {
		if parsedQuery, parsedReason, ok := support.ParseRewriteDecision(msg.Content); ok && strings.TrimSpace(parsedQuery) != "" {
			query = strings.TrimSpace(parsedQuery)
			reason = parsedReason
		}
	}
	flow.Rewrite = orchestratorstate.RewriteDecision{Query: query, Reason: reason}
	flow.State.RewrittenQuery = query
	orchestratorstate.BindConversationFlow(ctx, flow)
	return flow, nil
}

func (n *RetrieveNode) PrepareQuery(ctx context.Context, flow *orchestratorstate.FlowContext) (string, error) {
	orchestratorstate.BindConversationFlow(ctx, flow)
	if flow == nil {
		return "", fmt.Errorf("flow is required")
	}
	return strings.TrimSpace(support.FirstNonEmpty(flow.State.RewrittenQuery, flow.State.RawQuery, flow.Request.Message)), nil
}

func (n *RetrieveNode) ApplyDocuments(ctx context.Context, docs []*schema.Document) (*orchestratorstate.FlowContext, error) {
	flow := orchestratorstate.ConversationFlowFromContext(ctx)
	if flow == nil {
		return nil, fmt.Errorf("conversation flow is missing")
	}
	flow.Retrieval.Query = strings.TrimSpace(support.FirstNonEmpty(flow.State.RewrittenQuery, flow.State.RawQuery))
	flow.Retrieval.Documents = docs
	orchestratorstate.BindConversationFlow(ctx, flow)
	return flow, nil
}

func (n *RerankNode) Invoke(ctx context.Context, flow *orchestratorstate.FlowContext) (*orchestratorstate.FlowContext, error) {
	if flow == nil || len(flow.Retrieval.Documents) == 0 {
		orchestratorstate.BindConversationFlow(ctx, flow)
		return flow, nil
	}

	refs := make([]dto.KnowledgeRef, 0, len(flow.Retrieval.Documents))
	queryTokens := support.SplitTerms(support.FirstNonEmpty(flow.State.RewrittenQuery, flow.State.RawQuery))
	for _, doc := range flow.Retrieval.Documents {
		if doc == nil {
			continue
		}
		ref := dto.KnowledgeRef{
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
	if len(refs) > n.suite.deps.Config.RerankTopK {
		refs = refs[:n.suite.deps.Config.RerankTopK]
	}
	flow.Retrieval.References = refs
	orchestratorstate.BindConversationFlow(ctx, flow)
	return flow, nil
}

func (n *FallbackNode) Invoke(ctx context.Context, flow *orchestratorstate.FlowContext) (*orchestratorstate.FlowContext, error) {
	if flow == nil {
		return nil, fmt.Errorf("flow is required")
	}
	if strings.TrimSpace(flow.State.FinalAnswer) == "" {
		flow.State.FinalAnswer = support.FallbackAnswer(flow)
	}
	orchestratorstate.BindConversationFlow(ctx, flow)
	return flow, nil
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
