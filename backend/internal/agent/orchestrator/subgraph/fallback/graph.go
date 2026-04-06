package fallback

import (
	"context"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	orchestratornode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node"
	ragnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/rag"
)

// Input 描述兜底子图的入口。
type Input struct {
	RawQuery    string
	Intent      string
	History     []*schema.Message
	FinalAnswer string
}

// Output 描述兜底子图的出口。
type Output struct {
	FinalAnswer string
	Query       string
	Documents   []*schema.Document
}

// Build 组装兜底子图。
// 它会优先尝试知识库检索；如果没有形成明确回答，再走兜底文案生成。
func Build(_ context.Context, ragNode *ragnode.RAGNode, fallbackNode *orchestratornode.FallbackNode) (compose.AnyGraph, error) {
	g := compose.NewGraph[Input, Output]()
	if err := g.AddLambdaNode("ExecuteFallbackFlowNode", compose.InvokableLambda(
		func(ctx context.Context, input Input) (Output, error) {
			out := Output{FinalAnswer: input.FinalAnswer}
			if ragNode != nil {
				ragResult, err := ragNode.Invoke(ctx, ragnode.Input{
					Message: input.RawQuery,
					History: append([]*schema.Message(nil), input.History...),
					Intent:  input.Intent,
				})
				if err != nil {
					return Output{}, err
				}
				if ragResult != nil {
					out.Query = ragResult.Query
					out.Documents = append([]*schema.Document(nil), ragResult.Documents...)
				}
			}

			if fallbackNode != nil {
				result, err := fallbackNode.Invoke(ctx, orchestratornode.FallbackInput{
					FinalAnswer: out.FinalAnswer,
					Documents:   append([]*schema.Document(nil), out.Documents...),
				})
				if err != nil {
					return Output{}, err
				}
				if result != nil {
					out.FinalAnswer = result.FinalAnswer
				}
			}
			return out, nil
		}), compose.WithNodeName("ExecuteFallbackFlowNode")); err != nil {
		return nil, err
	}
	if err := g.AddEdge(compose.START, "ExecuteFallbackFlowNode"); err != nil {
		return nil, err
	}
	if err := g.AddEdge("ExecuteFallbackFlowNode", compose.END); err != nil {
		return nil, err
	}
	return g, nil
}
