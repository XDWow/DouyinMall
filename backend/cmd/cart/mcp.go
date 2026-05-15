package main

import (
	"fmt"
	"net/http"
	"strings"

	cartmcp "github.com/XDWow/DouyinMall/backend/internal/cart/transport/mcp"
	kitexclient "github.com/cloudwego/kitex/client"

	cartservice "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/cart/v1/cartservice"
)

func newMCPHandler(cfg cartmcp.Config) (http.Handler, error) {
	serviceName := strings.TrimSpace(cfg.Upstream.ServiceName)
	if serviceName == "" {
		serviceName = "cart.service"
	}

	directAddr := strings.TrimSpace(cfg.Upstream.DirectAddr)
	if directAddr == "" {
		directAddr = "127.0.0.1:8099"
	}

	client, err := cartservice.NewClient(serviceName, kitexclient.WithHostPorts(directAddr))
	if err != nil {
		return nil, fmt.Errorf("init cart MCP upstream client failed: %w", err)
	}
	return cartmcp.NewServer(cfg, client)
}
