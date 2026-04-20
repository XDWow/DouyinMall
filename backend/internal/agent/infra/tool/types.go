package tool

import (
	"time"

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
