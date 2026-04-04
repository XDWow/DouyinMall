package tool

import (
	"encoding/json"
	"time"

	einotool "github.com/cloudwego/eino/components/tool"

	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
)

type OrderSummary struct {
	OrderID     int64     `json:"order_id"`
	Status      string    `json:"status"`
	TotalAmount int64     `json:"total_amount"`
	Currency    string    `json:"currency"`
	CreatedAt   time.Time `json:"created_at"`
}

type ProductSummary struct {
	ID           int64    `json:"id"`
	Name         string   `json:"name"`
	Price        int64    `json:"price"`
	Categories   []string `json:"categories,omitempty"`
	MerchantName string   `json:"merchant_name,omitempty"`
	InStock      bool     `json:"in_stock"`
}

type ExecutionRecorder interface {
	Record(exec domain.ToolExecution)
}

type ToolExecutionMode string

const (
	ToolExecutionSerial           ToolExecutionMode = "serial"
	ToolExecutionParallelReadOnly ToolExecutionMode = "parallel_readonly"
)

type ToolPolicy struct {
	ReadOnly         bool
	RequiresOrdering bool
}

type registeredInvokableTool struct {
	invokable einotool.InvokableTool
	policy    ToolPolicy
}

func toolArgumentsJSON(plan domain.ToolCallPlan) (string, error) {
	if plan.RawJSON != "" {
		return plan.RawJSON, nil
	}
	payload, err := json.Marshal(plan.Arguments)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}
