package graph

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	agentskill "github.com/XDWow/DouyinMall/backend/internal/agent/infra/skill"
	agenttool "github.com/XDWow/DouyinMall/backend/internal/agent/infra/tool"
	globalnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/global"
	understandingnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/global/understanding"
	ragnode "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/node/shared/rag"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/addtocart"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/aftersalesapply"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/aftersalespolicy"
	subgraphcommon "github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/common"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/orderservice"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/productservice"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/promotionservice"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/subgraph/unknown"
	"github.com/XDWow/DouyinMall/backend/internal/agent/orchestrator/support"
)

type Config struct {
	InterruptBeforeNodes []string
}

type Builder struct {
	Config          Config
	CheckpointStore compose.CheckPointStore
	Registry        *agenttool.Registry
	Skills          *agentskill.Registry
	AgentModel      model.ToolCallingChatModel

	AccessGuard   *globalnode.AccessGuardNode
	SessionLoad   *globalnode.SessionLoadNode
	Understanding *understandingnode.UnderstandingNode
	Route         *globalnode.RouteNode
	Finalize      *globalnode.FinalizeNode

	RAG *ragnode.RAGNode
}

func (b *Builder) Build(ctx context.Context) (compose.Runnable[struct{}, *domain.State], error) {
	// 主图固定骨架：准入 -> 会话 -> 理解 -> 路由 -> 子图 -> 收口
	wf := compose.NewWorkflow[struct{}, *domain.State](
		compose.WithGenLocalState(domain.SharedGraphState),
	)

	if err := b.addGlobalNodes(wf); err != nil {
		return nil, err
	}
	if err := b.addSubgraphs(ctx, wf); err != nil {
		return nil, err
	}
	if err := b.addBranches(wf); err != nil {
		return nil, err
	}

	opts := []compose.GraphCompileOption{
		compose.WithGraphName("agent_graph"),
	}
	if b.CheckpointStore != nil {
		opts = append(opts, compose.WithCheckPointStore(b.CheckpointStore))
	}
	if len(b.Config.InterruptBeforeNodes) > 0 {
		opts = append(opts, compose.WithInterruptBeforeNodes(b.Config.InterruptBeforeNodes))
	}
	return wf.Compile(ctx, opts...)
}

func (b *Builder) addGlobalNodes(wf *compose.Workflow[struct{}, *domain.State]) error {
	wf.AddLambdaNode("AccessGuardNode",
		compose.InvokableLambda(b.AccessGuard.Invoke),
		compose.WithStatePreHandler(func(_ context.Context, in globalnode.AccessGuardInput, st *domain.State) (globalnode.AccessGuardInput, error) {
			if st == nil || st.Input == nil {
				return in, fmt.Errorf("state input is required")
			}
			return globalnode.AccessGuardInput{
				UserID:      st.Input.UserID,
				Message:     strings.TrimSpace(st.Input.Message),
				ResumeToken: strings.TrimSpace(st.Input.ResumeToken),
			}, nil
		}),
		compose.WithStatePostHandler(func(_ context.Context, out globalnode.AccessGuardResult, st *domain.State) (globalnode.AccessGuardResult, error) {
			if st != nil && out.Response != nil {
				resp := *out.Response
				st.Response = &resp
			}
			return out, nil
		}),
	).AddDependency(compose.START)

	wf.AddLambdaNode("SessionLoadNode",
		compose.InvokableLambda(b.SessionLoad.Invoke),
		compose.WithStatePreHandler(func(_ context.Context, in globalnode.SessionLoadInput, st *domain.State) (globalnode.SessionLoadInput, error) {
			if st == nil || st.Input == nil {
				return in, fmt.Errorf("state input is required")
			}
			return globalnode.SessionLoadInput{
				SessionID: strings.TrimSpace(st.Session.SessionID),
				UserID:    st.Input.UserID,
			}, nil
		}),
		compose.WithStatePostHandler(func(_ context.Context, out *domain.Session, st *domain.State) (*domain.Session, error) {
			if st != nil && out != nil {
				st.Session = out
			}
			return out, nil
		}),
	).AddDependency("AccessGuardNode")

	wf.AddLambdaNode("UnderstandingNode",
		compose.InvokableLambda(b.Understanding.Invoke),
		compose.WithStatePreHandler(func(_ context.Context, in understandingnode.UnderstandingInput, st *domain.State) (understandingnode.UnderstandingInput, error) {
			if st == nil || st.Input == nil {
				return in, fmt.Errorf("state input is required")
			}
			history := []string(nil)
			if st.Session != nil {
				history = subgraphcommon.HistoryLines(st.Session.RecentMessages)
			}
			return understandingnode.UnderstandingInput{
				UserMessage:   strings.TrimSpace(st.Input.Message),
				RecentHistory: history,
			}, nil
		}),
		compose.WithStatePostHandler(func(_ context.Context, out *understandingnode.UnderstandingResult, st *domain.State) (*understandingnode.UnderstandingResult, error) {
			if st == nil || out == nil {
				return out, nil
			}
			// 主图统一回写路由级理解结果；后续子图直接消费这些共享上下文。
			st.Intent = domain.Intent(out.Intent)
			st.RewrittenQuery = strings.TrimSpace(out.RewrittenQuery)
			if st.Session == nil {
				st.Session = &domain.Session{}
			}
			if st.Session.Slots == nil {
				st.Session.Slots = make(map[string]any)
			}
			support.MergeSlots(st.Session.Slots, out.Slots)
			return out, nil
		}),
	).AddDependency("SessionLoadNode")

	wf.AddLambdaNode("RouteNode", compose.InvokableLambda(b.Route.Invoke)).
		AddInput("UnderstandingNode")

	// Finalize 只收各子图的最终产物，不再参与中间业务决策
	wf.AddLambdaNode("FinalizeNode", compose.InvokableLambda(b.Finalize.Invoke)).
		AddInputWithOptions("AccessGuardNode", []*compose.FieldMapping{
			compose.MapFields("Response", "AccessGuard"),
		}, compose.WithNoDirectDependency()).
		AddInput("ProductServiceGraph", compose.ToField("ProductService")).
		AddInput("OrderServiceGraph", compose.ToField("OrderService")).
		AddInput("PromotionServiceGraph", compose.ToField("PromotionService")).
		AddInput("AftersalesPolicyGraph", compose.ToField("AftersalesPolicy")).
		AddInput("AftersalesApplyGraph", compose.ToField("AftersalesApply")).
		AddInput("AddToCartGraph", compose.ToField("AddToCart")).
		AddInput("UnknownGraph", compose.ToField("Unknown"))

	wf.End().AddInput("FinalizeNode")
	return nil
}

func (b *Builder) addSubgraphs(ctx context.Context, wf *compose.Workflow[struct{}, *domain.State]) error {
	// 读型子图走 Agent/RAG，写型子图保持确定性流程
	productGraph, err := productservice.Build(ctx, b.AgentModel, b.Registry, b.Skills)
	if err != nil {
		return err
	}
	orderGraph, err := orderservice.Build(ctx, b.AgentModel, b.Registry, b.Skills)
	if err != nil {
		return err
	}
	promotionGraph, err := promotionservice.Build(ctx, b.AgentModel, b.Registry, b.Skills, b.RAG)
	if err != nil {
		return err
	}
	policyGraph, err := aftersalespolicy.Build(ctx, b.AgentModel, b.Registry, b.Skills, b.RAG)
	if err != nil {
		return err
	}
	applyGraph, err := aftersalesapply.Build(ctx, b.AgentModel, b.Registry, b.Skills)
	if err != nil {
		return err
	}
	addToCartGraph, err := addtocart.Build(ctx, b.AgentModel, b.Registry, b.Skills)
	if err != nil {
		return err
	}
	unknownGraph, err := unknown.Build(ctx)
	if err != nil {
		return err
	}

	if productGraph != nil {
		wf.AddGraphNode("ProductServiceGraph", productGraph)
	}
	if orderGraph != nil {
		wf.AddGraphNode("OrderServiceGraph", orderGraph)
	}
	if promotionGraph != nil {
		wf.AddGraphNode("PromotionServiceGraph", promotionGraph)
	}
	if policyGraph != nil {
		wf.AddGraphNode("AftersalesPolicyGraph", policyGraph)
	}
	if applyGraph != nil {
		wf.AddGraphNode("AftersalesApplyGraph", applyGraph)
	}
	if addToCartGraph != nil {
		wf.AddGraphNode("AddToCartGraph", addToCartGraph)
	}
	if unknownGraph != nil {
		wf.AddGraphNode("UnknownGraph", unknownGraph)
	}
	return nil
}

func (b *Builder) addBranches(wf *compose.Workflow[struct{}, *domain.State]) error {
	wf.AddBranch("AccessGuardNode", compose.NewGraphBranch(
		func(_ context.Context, in globalnode.AccessGuardResult) (string, error) {
			if in.Blocked {
				return "FinalizeNode", nil
			}
			return "SessionLoadNode", nil
		},
		map[string]bool{"SessionLoadNode": true, "FinalizeNode": true},
	))
	wf.AddBranch("RouteNode", compose.NewGraphBranch(
		branchFromRoute,
		map[string]bool{
			"ProductServiceGraph":   true,
			"OrderServiceGraph":     true,
			"PromotionServiceGraph": true,
			"AftersalesPolicyGraph": true,
			"AftersalesApplyGraph":  true,
			"AddToCartGraph":        true,
			"UnknownGraph":          true,
		},
	))
	return nil
}

func branchFromRoute(_ context.Context, in domain.WorkflowRoute) (string, error) {
	switch in {
	case domain.RouteProductService:
		return "ProductServiceGraph", nil
	case domain.RouteOrderService:
		return "OrderServiceGraph", nil
	case domain.RoutePromotionService:
		return "PromotionServiceGraph", nil
	case domain.RouteAftersalesPolicy:
		return "AftersalesPolicyGraph", nil
	case domain.RouteAftersalesApply:
		return "AftersalesApplyGraph", nil
	case domain.RouteAddToCart:
		return "AddToCartGraph", nil
	case domain.RouteUnknown:
		return "UnknownGraph", nil
	default:
		return "", fmt.Errorf("unsupported route: %s", in)
	}
}
