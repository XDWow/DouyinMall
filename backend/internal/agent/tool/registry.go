//go:build legacy_agent

package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	einotool "github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/compose"

	"github.com/XDWow/DouyinMall/backend/internal/agent/dto"
)

type Registry struct {
	tools []einotool.BaseTool
	node  *compose.ToolsNode
}

func NewRegistry(ctx context.Context, gateway Gateway) (*Registry, error) {
	queryOrderTool, err := toolutils.InferTool(
		"query_order",
		"查询当前用户订单。适用于询问订单状态、最近订单、某个订单是否已创建或何时下单。",
		func(ctx context.Context, input QueryOrderArgs) (QueryOrderResult, error) {
			runtime := runtimeFromContext(ctx)
			if runtime.UserID <= 0 {
				return QueryOrderResult{}, fmt.Errorf("tool runtime user_id is required")
			}
			limit := input.Limit
			if limit <= 0 {
				limit = 5
			}
			orders, err := gateway.QueryOrders(ctx, runtime.UserID, input.OrderID, limit)
			if err != nil {
				return QueryOrderResult{}, err
			}
			return QueryOrderResult{Orders: orders}, nil
		},
	)
	if err != nil {
		return nil, err
	}

	searchProductTool, err := toolutils.InferTool(
		"search_product",
		"搜索商品。适用于用户想找某类商品、比较商品、询问有哪些商品可买。",
		func(ctx context.Context, input SearchProductArgs) (SearchProductResult, error) {
			limit := input.Limit
			if limit <= 0 {
				limit = 5
			}
			products, err := gateway.SearchProducts(ctx, input.Query, limit)
			if err != nil {
				return SearchProductResult{}, err
			}
			return SearchProductResult{Products: products}, nil
		},
	)
	if err != nil {
		return nil, err
	}

	addToCartTool, err := toolutils.InferTool(
		"add_to_cart",
		"加购物车。适用于用户明确表达想把某个商品加入购物车，必须给出 product_id。",
		func(ctx context.Context, input AddToCartArgs) (AddToCartResult, error) {
			runtime := runtimeFromContext(ctx)
			if runtime.UserID <= 0 {
				return AddToCartResult{}, fmt.Errorf("tool runtime user_id is required")
			}
			if input.Quantity <= 0 {
				input.Quantity = 1
			}
			if err := gateway.AddToCart(ctx, runtime.UserID, input.ProductID, input.Quantity); err != nil {
				return AddToCartResult{}, err
			}
			return AddToCartResult{
				Success:   true,
				ProductID: input.ProductID,
				Quantity:  input.Quantity,
			}, nil
		},
	)
	if err != nil {
		return nil, err
	}

	tools := []einotool.BaseTool{queryOrderTool, searchProductTool, addToCartTool}
	toolNode, err := compose.NewToolNode(ctx, &compose.ToolsNodeConfig{
		Tools: tools,
		ToolCallMiddlewares: []compose.ToolMiddleware{
			{Invokable: recordInvokableToolExecution},
		},
	})
	if err != nil {
		return nil, err
	}

	return &Registry{
		tools: tools,
		node:  toolNode,
	}, nil
}

func (r *Registry) Tools() []einotool.BaseTool {
	return r.tools
}

func (r *Registry) Node() *compose.ToolsNode {
	return r.node
}

func recordInvokableToolExecution(next compose.InvokableToolEndpoint) compose.InvokableToolEndpoint {
	return func(ctx context.Context, input *compose.ToolInput) (*compose.ToolOutput, error) {
		start := time.Now()
		output, err := next(ctx, input)
		recorder := executionRecorderFromContext(ctx)
		if recorder == nil {
			return output, err
		}

		args := make(map[string]any)
		if input != nil && input.Arguments != "" {
			_ = json.Unmarshal([]byte(input.Arguments), &args)
		}

		exec := dto.ToolExecution{
			Name:       input.Name,
			Arguments:  args,
			Success:    err == nil,
			LatencyMs:  time.Since(start).Milliseconds(),
			OccurredAt: start,
		}
		if output != nil {
			exec.Result = output.Result
		}
		if err != nil {
			exec.Error = err.Error()
		}
		recorder.Record(exec)
		return output, err
	}
}

type SafeExecutionRecorder struct {
	mu    sync.Mutex
	items []dto.ToolExecution
}

func NewSafeExecutionRecorder() *SafeExecutionRecorder {
	return &SafeExecutionRecorder{}
}

func (r *SafeExecutionRecorder) Record(exec dto.ToolExecution) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items = append(r.items, exec)
}

func (r *SafeExecutionRecorder) Snapshot() []dto.ToolExecution {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]dto.ToolExecution, len(r.items))
	copy(out, r.items)
	return out
}

type QueryOrderArgs struct {
	UserID  int64 `json:"user_id" jsonschema_description:"当前用户ID"`
	OrderID int64 `json:"order_id,omitempty" jsonschema_description:"可选的订单ID"`
	Limit   int   `json:"limit,omitempty" jsonschema_description:"返回订单条数"`
}

type QueryOrderResult struct {
	Orders []OrderSummary `json:"orders"`
}

type SearchProductArgs struct {
	Query string `json:"query" jsonschema_description:"商品搜索关键词"`
	Limit int    `json:"limit,omitempty" jsonschema_description:"返回商品数量"`
}

type SearchProductResult struct {
	Products []ProductSummary `json:"products"`
}

type AddToCartArgs struct {
	UserID    int64 `json:"user_id" jsonschema_description:"当前用户ID"`
	ProductID int64 `json:"product_id" jsonschema_description:"商品ID"`
	Quantity  int64 `json:"quantity,omitempty" jsonschema_description:"加入购物车数量"`
}

type AddToCartResult struct {
	Success   bool  `json:"success"`
	ProductID int64 `json:"product_id"`
	Quantity  int64 `json:"quantity"`
}

func unixMilliToTime(ts int64) time.Time {
	if ts <= 0 {
		return time.Time{}
	}
	if ts > 1e12 {
		return time.UnixMilli(ts)
	}
	return time.Unix(ts, 0)
}

func ParseToolArguments(raw string) map[string]any {
	if raw == "" {
		return nil
	}
	m := make(map[string]any)
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return map[string]any{"raw": raw, "error": fmt.Sprintf("invalid json: %v", err)}
	}
	return m
}
