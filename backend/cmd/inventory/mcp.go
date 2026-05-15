package main

import (
	"fmt"
	"net/http"
	"strings"

	inventorymcp "github.com/XDWow/DouyinMall/backend/internal/inventory/transport/mcp"
	kitexclient "github.com/cloudwego/kitex/client"

	inventoryservice "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/inventory/v1/inventoryservice"
)

func newMCPHandler(cfg inventorymcp.Config) (http.Handler, error) {
	serviceName := strings.TrimSpace(cfg.Upstream.ServiceName)
	if serviceName == "" {
		serviceName = "inventory.service"
	}

	directAddr := strings.TrimSpace(cfg.Upstream.DirectAddr)
	if directAddr == "" {
		directAddr = "127.0.0.1:8094"
	}

	client, err := inventoryservice.NewClient(serviceName, kitexclient.WithHostPorts(directAddr))
	if err != nil {
		return nil, fmt.Errorf("init inventory MCP upstream client failed: %w", err)
	}
	return inventorymcp.NewServer(cfg, client)
}
