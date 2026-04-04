package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	aftersaleconfig "github.com/XDWow/DouyinMall/backend/internal/aftersale/config"
	"github.com/XDWow/DouyinMall/backend/pkg/mcpruntime"
	aftersalev1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/aftersale/v1"
	aftersaleservice "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/aftersale/v1/aftersaleservice"
	mcpproto "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

type Adapter struct {
	client aftersaleservice.Client
}

func NewServer(cfg aftersaleconfig.MCPConfig, client aftersaleservice.Client) (http.Handler, error) {
	cfg = applyDefaults(cfg)
	adapter := &Adapter{client: client}
	server := mcpserver.NewMCPServer(
		cfg.Server.Name,
		cfg.Server.Version,
		mcpserver.WithToolCapabilities(false),
		mcpserver.WithRecovery(),
	)

	for _, tool := range cfg.Tools {
		if !tool.Enabled {
			continue
		}
		switch tool.Key {
		case "create_after_sale_request":
			server.AddTool(mcpproto.NewTool(
				tool.Name,
				mcpproto.WithDescription(tool.Description),
				mcpproto.WithNumber("order_id", mcpproto.Description("Order ID"), mcpproto.Required()),
				mcpproto.WithNumber("item_id", mcpproto.Description("Optional item or SKU ID")),
				mcpproto.WithString("reason", mcpproto.Description("After-sale reason"), mcpproto.Required()),
				mcpproto.WithString("request_type", mcpproto.Description("After-sale request type"), mcpproto.Enum("return", "exchange"), mcpproto.DefaultString("return")),
			), adapter.CreateAfterSaleRequest)
		case "get_after_sale_request":
			server.AddTool(mcpproto.NewTool(
				tool.Name,
				mcpproto.WithDescription(tool.Description),
				mcpproto.WithString("request_no", mcpproto.Description("After-sale request number"), mcpproto.Required()),
			), adapter.GetAfterSaleRequest)
		}
	}

	return mcpserver.NewStreamableHTTPServer(
		server,
		mcpserver.WithHTTPContextFunc(mcpruntime.WithHTTPContext),
	), nil
}

func (a *Adapter) CreateAfterSaleRequest(ctx context.Context, req mcpproto.CallToolRequest) (*mcpproto.CallToolResult, error) {
	runtime := mcpruntime.FromContext(ctx)
	if runtime.UserID <= 0 {
		return mcpproto.NewToolResultError("missing runtime user_id"), nil
	}

	var args struct {
		OrderID     int64          `json:"order_id"`
		ItemID      int64          `json:"item_id"`
		Reason      string         `json:"reason"`
		RequestType string         `json:"request_type"`
		Metadata    map[string]any `json:"metadata"`
	}
	if err := req.BindArguments(&args); err != nil {
		return mcpproto.NewToolResultError("invalid arguments: " + err.Error()), nil
	}
	metadataJSON, _ := json.Marshal(args.Metadata)

	resp, err := a.client.CreateAfterSaleRequest(ctx, &aftersalev1.CreateAfterSaleRequestReq{
		UserId:       runtime.UserID,
		OrderId:      args.OrderID,
		ItemId:       args.ItemID,
		RequestType:  stringToRequestType(args.RequestType),
		Reason:       args.Reason,
		SessionId:    runtime.SessionID,
		TraceId:      runtime.TraceID,
		MetadataJson: string(metadataJSON),
	})
	if err != nil {
		return mcpproto.NewToolResultError("create after sale request failed: " + err.Error()), nil
	}
	request := resp.GetRequest()
	if request == nil {
		return mcpproto.NewToolResultError("empty after sale response"), nil
	}

	return mcpproto.NewToolResultText(toJSON(map[string]any{
		"request_no":   request.GetRequestNo(),
		"status":       requestStatusToString(request.GetStatus()),
		"request_type": requestTypeToString(request.GetRequestType()),
		"order_id":     request.GetOrderId(),
		"item_id":      request.GetItemId(),
		"reason":       request.GetReason(),
		"created_at":   request.GetCreatedAt(),
	})), nil
}

func (a *Adapter) GetAfterSaleRequest(ctx context.Context, req mcpproto.CallToolRequest) (*mcpproto.CallToolResult, error) {
	runtime := mcpruntime.FromContext(ctx)
	if runtime.UserID <= 0 {
		return mcpproto.NewToolResultError("missing runtime user_id"), nil
	}

	var args struct {
		RequestNo string `json:"request_no"`
	}
	if err := req.BindArguments(&args); err != nil {
		return mcpproto.NewToolResultError("invalid arguments: " + err.Error()), nil
	}

	resp, err := a.client.GetAfterSaleRequest(ctx, &aftersalev1.GetAfterSaleRequestReq{
		RequestNo: args.RequestNo,
	})
	if err != nil {
		return mcpproto.NewToolResultError("get after sale request failed: " + err.Error()), nil
	}
	request := resp.GetRequest()
	if request == nil {
		return mcpproto.NewToolResultError("after sale request not found"), nil
	}
	if request.GetUserId() != runtime.UserID {
		return mcpproto.NewToolResultError("after sale request does not belong to current user"), nil
	}

	return mcpproto.NewToolResultText(toJSON(map[string]any{
		"request_no":   request.GetRequestNo(),
		"status":       requestStatusToString(request.GetStatus()),
		"request_type": requestTypeToString(request.GetRequestType()),
		"order_id":     request.GetOrderId(),
		"item_id":      request.GetItemId(),
		"reason":       request.GetReason(),
		"created_at":   request.GetCreatedAt(),
	})), nil
}

func applyDefaults(cfg aftersaleconfig.MCPConfig) aftersaleconfig.MCPConfig {
	if strings.TrimSpace(cfg.Server.Name) == "" {
		cfg.Server.Name = "aftersale-mcp"
	}
	if strings.TrimSpace(cfg.Server.Version) == "" {
		cfg.Server.Version = "1.0.0"
	}
	if len(cfg.Tools) == 0 {
		cfg.Tools = []aftersaleconfig.ToolConfig{
			{
				Key:         "create_after_sale_request",
				Name:        "create_after_sale_request",
				Description: "Create an after-sale request for return or exchange.",
				Enabled:     true,
			},
			{
				Key:         "get_after_sale_request",
				Name:        "get_after_sale_request",
				Description: "Get after-sale request details by request number.",
				Enabled:     true,
			},
		}
	}
	return cfg
}

func stringToRequestType(value string) aftersalev1.AfterSaleRequestType {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "exchange":
		return aftersalev1.AfterSaleRequestType_AFTER_SALE_REQUEST_TYPE_EXCHANGE
	default:
		return aftersalev1.AfterSaleRequestType_AFTER_SALE_REQUEST_TYPE_RETURN
	}
}

func requestTypeToString(value aftersalev1.AfterSaleRequestType) string {
	switch value {
	case aftersalev1.AfterSaleRequestType_AFTER_SALE_REQUEST_TYPE_EXCHANGE:
		return "exchange"
	default:
		return "return"
	}
}

func requestStatusToString(value aftersalev1.AfterSaleRequestStatus) string {
	switch value {
	case aftersalev1.AfterSaleRequestStatus_AFTER_SALE_REQUEST_STATUS_PENDING_REVIEW:
		return "pending_review"
	case aftersalev1.AfterSaleRequestStatus_AFTER_SALE_REQUEST_STATUS_APPROVED:
		return "approved"
	case aftersalev1.AfterSaleRequestStatus_AFTER_SALE_REQUEST_STATUS_REJECTED:
		return "rejected"
	case aftersalev1.AfterSaleRequestStatus_AFTER_SALE_REQUEST_STATUS_CANCELED:
		return "canceled"
	case aftersalev1.AfterSaleRequestStatus_AFTER_SALE_REQUEST_STATUS_COMPLETED:
		return "completed"
	default:
		return "unknown"
	}
}

func toJSON(v any) string {
	data, _ := json.Marshal(v)
	return string(data)
}


