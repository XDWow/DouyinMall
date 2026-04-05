package fallback

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/model"
	einoretriever "github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/compose"

	orchestratorprompt "github.com/XDWow/DouyinMall/backend/internal/agent/components/prompt"
	orchestratornode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node"
	orchestratorstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/returnpolicy"
)

// Build 构建兜底子图。优先复用 RAG 知识库兜底；若 RAG 依赖不可用，则退化为纯规则回复。
func Build(
	ctx context.Context,
	chatModel model.ToolCallingChatModel,
	retriever einoretriever.Retriever,
	prompts *orchestratorprompt.Set,
	rewriteNode *orchestratornode.RewriteNode,
	retrieveNode *orchestratornode.RetrieveNode,
	rerankNode *orchestratornode.RerankNode,
	fallbackNode *orchestratornode.FallbackNode,
) (compose.AnyGraph, error) {
	// 尝试用退换政策 RAG 子图兜底——共享同一套检索链路，避免重复建图
	ragGraph, err := returnpolicy.Build(ctx, chatModel, retriever, prompts, rewriteNode, retrieveNode, rerankNode)
	if err != nil {
		return nil, err
	}
	if ragGraph != nil {
		return ragGraph, nil
	}

	// RAG 依赖不可用，退化为规则兜底节点
	g := compose.NewGraph[*orchestratorstate.ConversationState, *orchestratorstate.ConversationState]()
	if err := g.AddLambdaNode("FallbackResolveNode", compose.InvokableLambda(fallbackNode.Invoke), compose.WithNodeName("FallbackResolveNode")); err != nil {
		return nil, err
	}
	if err := addEdge(g, compose.START, "FallbackResolveNode"); err != nil {
		return nil, err
	}
	if err := addEdge(g, "FallbackResolveNode", compose.END); err != nil {
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
