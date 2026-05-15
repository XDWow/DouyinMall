package main

import (
	"fmt"
	"net/http"
	"strings"

	aftersaleconfig "github.com/XDWow/DouyinMall/backend/internal/aftersale/config"
	aftersalemcp "github.com/XDWow/DouyinMall/backend/internal/aftersale/transport/mcp"
	kitexclient "github.com/cloudwego/kitex/client"

	aftersaleservice "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/aftersale/v1/aftersaleservice"
)

func newMCPHandler(cfg aftersaleconfig.Config) (http.Handler, error) {
	serviceName := strings.TrimSpace(cfg.MCP.Upstream.ServiceName)
	if serviceName == "" {
		serviceName = strings.TrimSpace(cfg.GRPC.Server.Name)
	}
	if serviceName == "" {
		serviceName = "aftersale.service"
	}

	directAddr := strings.TrimSpace(cfg.MCP.Upstream.DirectAddr)
	if directAddr == "" {
		port := cfg.GRPC.Server.Port
		if port == 0 {
			port = 8097
		}
		directAddr = fmt.Sprintf("127.0.0.1:%d", port)
	}

	client, err := aftersaleservice.NewClient(serviceName, kitexclient.WithHostPorts(directAddr))
	if err != nil {
		return nil, fmt.Errorf("init aftersale MCP upstream client failed: %w", err)
	}

	return aftersalemcp.NewServer(cfg.MCP, client)
}
