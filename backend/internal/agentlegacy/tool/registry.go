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
		"闁哄被鍎撮妤勩亹閹惧啿顤呴柣顫妽閸╂稓鎷嬮姀鐘茬闁靛棗鍊块埀顒€鍊婚弫銈嗙鎼淬値鍤勯梻鍌ゅ枦椤撳綊宕￠弴鐘残﹂柟顑块檷閳ь兛鐒﹀〒鑸垫交閹达綆鍚傞柛妤佹磸閳ь兛鐒﹂悡鍥ㄧ▔椤忓浂鍚傞柛妤佹礃濡叉悂宕ラ敃鈧崙锟犲礆濞戞绱﹂柟瀛樼墧缂嶅秹寮張鐢电憮闁告娲忛埀?,
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
		"闁瑰吋绮庨崒銊╁疮閸℃鎯傞柕鍡楀€块埀顒€鍊婚弫銈嗙鎼达絾鏆忛柟鎾敱閸忓倿骞嶉悙顒傚帣缂侇偉顕ч弲銏ゅ传娴ｇ鍋撴担鍦Х閺夊牆鍟弲銏ゅ传娴ｇ鍋撴担绛嬪殑闂傚偆鍠楀﹢渚€宕鍐槀闁哥喎妫楅幖褔宕ｉ娆愬闁?,
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
		"闁告梻濮鹃崰姗€鎮ч埡鍕盃闁靛棗鍊块埀顒€鍊婚弫銈嗙鎼达絾鏆忛柟鎾敱濡叉垹娑甸瑁も偓鍐╂綇閻愵剙鍘掗柟璺猴攻閻撳洦绋夐鍕珜闁告繀绀佹慨鐐哄礂閵夈劌鏋犻柣妞绘櫈濠у懘鏁嶇仦鐣岀畱濡炪倛宕电划浼村礄?product_id闁?,
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
	UserID  int64 `json:"user_id" jsonschema_description:"鐟滅増鎸告晶鐘绘偨閵婏箑鐓旾D"`
	OrderID int64 `json:"order_id,omitempty" jsonschema_description:"闁告瑯鍨堕埀顒€顦卞▓鎴犳媼閵忕姴绀婭D"`
	Limit   int   `json:"limit,omitempty" jsonschema_description:"閺夆晜鏌ㄥú鏍媼閵忕姴绀嬮柡澶嗗墲閺?`
}

type QueryOrderResult struct {
	Orders []OrderSummary `json:"orders"`
}

type SearchProductArgs struct {
	Query string `json:"query" jsonschema_description:"闁哥喎妫楅幖褔骞栧鍛亶闁稿繑濞婇弫顓犳嫚?`
	Limit int    `json:"limit,omitempty" jsonschema_description:"閺夆晜鏌ㄥú鏍疮閸℃鎯傞柡浣峰嵆閸?`
}

type SearchProductResult struct {
	Products []ProductSummary `json:"products"`
}

type AddToCartArgs struct {
	UserID    int64 `json:"user_id" jsonschema_description:"鐟滅増鎸告晶鐘绘偨閵婏箑鐓旾D"`
	ProductID int64 `json:"product_id" jsonschema_description:"闁哥喎妫楅幖顪廌"`
	Quantity  int64 `json:"quantity,omitempty" jsonschema_description:"闁告梻濮撮崣鍡欐嫻椤撶姴鈷栭弶鐑囬檮閺嗙喖鏌?`
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

