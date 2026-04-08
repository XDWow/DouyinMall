package productinfo

import (
	"context"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/infra/tool"
	productnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/domain/product"
	globalnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/global"
	sharednode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/shared"
	ragnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/shared/rag"
	orchestratorstate "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/state"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/toolexec"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

// Input 描述商品咨询子图的入口。
type Input struct {
	TenantID     string
	UserID       int64
	SessionID    string
	TraceID      string
	CheckpointID string
	Slots        map[string]any
	RawQuery     string
	History      []*schema.Message
	Intent       string
	Recorder     *agenttool.SafeExecutionRecorder
}

// Output 描述商品咨询子图的出口。
type Output struct {
	CacheHit      bool
	HitLevel      string
	Response      *domain.ChatResult
	FinalAnswer   string
	NeedHandoff   bool
	HandoffReason string
	ReadOnly      bool
	ToolMessages  []*schema.Message
	Query         string
	Documents     []*schema.Document
}

// Build 组装商品咨询子图。
// 这段流程会先走商品工具查询，再按需要补一段知识库检索。
func Build(
	_ context.Context,
	registry *agenttool.Registry,
	productNode *productnode.ProductInfoNode,
	ragNode *ragnode.RAGNode,
	l1Cache *globalnode.L1SemanticCacheNode,
) (compose.AnyGraph, error) {
	if productNode == nil {
		return nil, nil
	}

	toolExecNode := sharednode.NewToolExecNode(registry)
	g := compose.NewGraph[Input, Output]()
	if err := g.AddLambdaNode("ExecuteProductInfoFlowNode", compose.InvokableLambda(
		func(ctx context.Context, input Input) (Output, error) {
			slots := cloneSlots(input.Slots)
			if slots == nil {
				slots = map[string]any{}
			}

			policy := globalnode.ResolveSemanticCachePolicy(orchestratorstate.RouteProductInfo, input.RawQuery)
			if policy.AllowRead && l1Cache != nil {
				cacheResult, cacheErr := l1Cache.Invoke(ctx, globalnode.L1SemanticCacheInput{
					TenantID:     input.TenantID,
					UserID:       input.UserID,
					Query:        input.RawQuery,
					SessionID:    input.SessionID,
					TraceID:      input.TraceID,
					CheckpointID: input.CheckpointID,
					IntentBucket: policy.IntentBucket,
					Scope:        policy.Scope,
					AllowRead:    true,
				})
				if cacheErr != nil {
					return Output{}, cacheErr
				}
				if cacheResult != nil && cacheResult.CacheHit {
					return Output{
						CacheHit:    true,
						HitLevel:    cacheResult.HitLevel,
						Response:    cacheResult.Response,
						FinalAnswer: cacheResult.FinalAnswer,
					}, nil
				}
			}

			result, err := productNode.Invoke(ctx, productnode.ProductInfoInput{
				Slots:    slots,
				RawQuery: input.RawQuery,
			})
			if err != nil {
				return Output{}, err
			}

			out := Output{
				FinalAnswer:   result.FinalAnswer,
				NeedHandoff:   result.NeedHandoff,
				HandoffReason: result.HandoffReason,
				ReadOnly:      result.ReadOnly,
			}
			if len(result.Plans) > 0 && toolExecNode != nil {
				callMessage, callErr := toolexec.CreateToolCallMessage(result.Plans)
				if callErr != nil {
					return Output{}, callErr
				}
				messages, execErr := toolExecNode.Invoke(ctx, sharednode.ToolExecutionInput{
					Plans:       result.Plans,
					CallMessage: callMessage,
					Mode:        agenttool.ToolExecutionParallelReadOnly,
				})
				if execErr != nil {
					return Output{}, execErr
				}
				out.ToolMessages = append([]*schema.Message(nil), messages...)
				if input.Recorder != nil {
					support.HydrateToolResultsIntoSlots(slots, input.Recorder.Snapshot())
				}
			}

			if ragNode != nil && !out.NeedHandoff && support.IsAdvisoryProductInfo(input.RawQuery) {
				ragResult, ragErr := ragNode.Invoke(ctx, ragnode.Input{
					Message: input.RawQuery,
					History: append([]*schema.Message(nil), input.History...),
					Intent:  input.Intent,
				})
				if ragErr != nil {
					return Output{}, ragErr
				}
				if ragResult != nil {
					out.Query = ragResult.Query
					out.Documents = append([]*schema.Document(nil), ragResult.Documents...)
				}
			}
			return out, nil
		}), compose.WithNodeName("ExecuteProductInfoFlowNode")); err != nil {
		return nil, err
	}
	if err := g.AddEdge(compose.START, "ExecuteProductInfoFlowNode"); err != nil {
		return nil, err
	}
	if err := g.AddEdge("ExecuteProductInfoFlowNode", compose.END); err != nil {
		return nil, err
	}
	return g, nil
}

func cloneSlots(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	result := make(map[string]any, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}
