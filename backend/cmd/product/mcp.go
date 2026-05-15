package main

import (
	"fmt"
	"net/http"
	"strings"

	productmcp "github.com/XDWow/DouyinMall/backend/internal/product/transport/mcp"
	kitexclient "github.com/cloudwego/kitex/client"

	productservice "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/product/v1/productservice"
)

func newMCPHandler(cfg productmcp.Config) (http.Handler, error) {
	serviceName := strings.TrimSpace(cfg.Upstream.ServiceName)
	if serviceName == "" {
		serviceName = "product.service"
	}

	directAddr := strings.TrimSpace(cfg.Upstream.DirectAddr)
	if directAddr == "" {
		directAddr = "127.0.0.1:8096"
	}

	client, err := productservice.NewClient(serviceName, kitexclient.WithHostPorts(directAddr))
	if err != nil {
		return nil, fmt.Errorf("init product MCP upstream client failed: %w", err)
	}
	return productmcp.NewServer(cfg, client)
}
