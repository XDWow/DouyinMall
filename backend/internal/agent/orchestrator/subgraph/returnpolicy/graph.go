package returnpolicy

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/model"
	einoretriever "github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/compose"

	orchestratorprompt "github.com/XDWow/DouyinMall/backend/internal/agent/components/prompt"
	orchestratornode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node"
	orchestratorstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/retrieve"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/rewrite"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

// Build 构建退换货政策 RAG 子图：查询改写 → 向量检索 → 重排序。
// 若 rewrite/retrieve 依赖不可用则返回 nil，调用方需自行降级处理。
func Build(
	ctx context.Context,
	chatModel model.ToolCallingChatModel,
	retriever einoretriever.Retriever,
	prompts *orchestratorprompt.Set,
	rewriteNode *orchestratornode.RewriteNode,
	retrieveNode *orchestratornode.RetrieveNode,
	rerankNode *orchestratornode.RerankNode,
) (compose.AnyGraph, error) {
	rewriteGraph, err := rewrite.Build(ctx, chatModel, prompts, rewriteNode)
	if err != nil {
		return nil, err
	}
	retrieveGraph, err := retrieve.Build(ctx, retriever, retrieveNode)
	if err != nil {
		return nil, err
	}
	if rewriteGraph == nil || retrieveGraph == nil {
		return nil, nil
	}

	g := compose.NewGraph[*orchestratorstate.ConversationState, *orchestratorstate.ConversationState]()

	// 入口节点：将 state 绑定到 ctx，供后续节点通过 context 访问
	if err := g.AddLambdaNode("ReturnPolicyStartNode", compose.InvokableLambda(
		func(ctx context.Context, state *orchestratorstate.ConversationState) (*orchestratorstate.ConversationState, error) {
			orchestratorstate.BindConversationState(ctx, state)
			return state, nil
		}), compose.WithNodeName("ReturnPolicyStartNode")); err != nil {
		return nil, err
	}
	if err := g.AddGraphNode("QueryRewriteNode", rewriteGraph, compose.WithNodeName("QueryRewriteNode")); err != nil {
		return nil, err
	}
	if err := g.AddGraphNode("KnowledgeRetrieverNode", retrieveGraph, compose.WithNodeName("KnowledgeRetrieverNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("RerankNode", compose.InvokableLambda(rerankNode.Invoke), compose.WithNodeName("RerankNode")); err != nil {
		return nil, err
	}

	for _, edge := range [][2]string{
		{compose.START, "ReturnPolicyStartNode"},
		{"KnowledgeRetrieverNode", "RerankNode"},
		{"RerankNode", compose.END},
	} {
		if err := addEdge(g, edge[0], edge[1]); err != nil {
			return nil, err
		}
	}

	// 若用户消息为空则直接结束，否则进入改写流程
	if err := g.AddBranch("ReturnPolicyStartNode", compose.NewGraphBranch(
		func(ctx context.Context, _ *orchestratorstate.ConversationState) (string, error) {
			state := orchestratorstate.ConversationStateFromContext(ctx)
			if state != nil && strings.TrimSpace(support.FirstNonEmpty(state.Session.RawQuery, state.Request.Message)) != "" {
				return "QueryRewriteNode", nil
			}
			return compose.END, nil
		},
		map[string]bool{"QueryRewriteNode": true, compose.END: true},
	)); err != nil {
		return nil, err
	}
	if err := addEdge(g, "QueryRewriteNode", "KnowledgeRetrieverNode"); err != nil {
		return nil, err
	}
	return g, nil
}

func addEdge(g interface{ AddEdge(string, string) error }, start, end string) error {
	if err := g.AddEdge(start, end); err != nil {
		return fmt.Errorf("添加边 %s -> %s 失败: %w", start, end, err)
	}
	return nil
}
