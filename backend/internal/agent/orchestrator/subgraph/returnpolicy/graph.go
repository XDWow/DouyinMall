package returnpolicy

import (
	"context"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	agentskill "github.com/XDWow/DouyinMall/backend/internal/agent/infra/skill"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/infra/tool"
	globalnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/global"
	sharednode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/shared"
	ragnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/shared/rag"
)

type Output struct {
	CacheHit    bool
	HitLevel    string
	Response    *domain.ChatResult
	FinalAnswer string
	Query       string
	Documents   []*schema.Document
}

func Build(
	_ context.Context,
	ragNode *ragnode.RAGNode,
	l1Cache *globalnode.L1SemanticCacheNode,
	chatModel model.ToolCallingChatModel,
	registry *agenttool.Registry,
	skills *agentskill.Registry,
	maxAnswerTokens int,
) (compose.AnyGraph, error) {
	if ragNode == nil && l1Cache == nil {
		return nil, nil
	}

	agent := sharednode.NewSubgraphAgent(chatModel, registry, skills, maxAnswerTokens)
	g := compose.NewGraph[struct{}, Output]()

	if err := g.AddLambdaNode("ReturnPolicyL1TryNode", compose.InvokableLambda(returnPolicyL1Try(l1Cache, skills)),
		compose.WithNodeName("ReturnPolicyL1TryNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("ReturnPolicyL1OutputNode", compose.InvokableLambda(buildReturnPolicyL1Output),
		compose.WithNodeName("ReturnPolicyL1OutputNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("ReturnPolicyRAGNode", compose.InvokableLambda(returnPolicyRAG(ragNode)),
		compose.WithNodeName("ReturnPolicyRAGNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("ReturnPolicyModelAgentNode", compose.InvokableLambda(returnPolicyModelAgent(agent)),
		compose.WithNodeName("ReturnPolicyModelAgentNode")); err != nil {
		return nil, err
	}
	if err := g.AddLambdaNode("ReturnPolicyBuildOutputNode", compose.InvokableLambda(buildReturnPolicyFinalOutput),
		compose.WithNodeName("ReturnPolicyBuildOutputNode")); err != nil {
		return nil, err
	}

	if err := g.AddEdge(compose.START, "ReturnPolicyL1TryNode"); err != nil {
		return nil, err
	}
	if err := g.AddBranch("ReturnPolicyL1TryNode", compose.NewGraphBranch(
		branchAfterReturnPolicyL1,
		map[string]bool{
			"ReturnPolicyL1OutputNode": true,
			"ReturnPolicyRAGNode":      true,
		},
	)); err != nil {
		return nil, err
	}
	if err := g.AddEdge("ReturnPolicyL1OutputNode", compose.END); err != nil {
		return nil, err
	}
	if err := g.AddEdge("ReturnPolicyRAGNode", "ReturnPolicyModelAgentNode"); err != nil {
		return nil, err
	}
	if err := g.AddEdge("ReturnPolicyModelAgentNode", "ReturnPolicyBuildOutputNode"); err != nil {
		return nil, err
	}
	if err := g.AddEdge("ReturnPolicyBuildOutputNode", compose.END); err != nil {
		return nil, err
	}
	return g, nil
}
