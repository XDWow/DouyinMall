package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func main() {
	initConfig()

	srv := InitMCPServer()

	port := viper.GetInt("server.port")
	if port == 0 {
		port = 9090
	}

	mux := http.NewServeMux()
	mux.Handle("/mcp", srv)

	// 健康检查
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	addr := fmt.Sprintf(":%d", port)
	log.Printf("MCP Tool Server 启动: http://0.0.0.0%s/mcp\n", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("MCP Tool Server 启动失败: %v", err)
	}
}

func initConfig() {
	cFile := pflag.String("config",
		"internal/mcptools/config/dev.yaml",
		"配置文件路径")
	pflag.Parse()

	if envConf := os.Getenv("MCP_CONFIG"); envConf != "" {
		*cFile = envConf
	}

	viper.SetConfigFile(*cFile)
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("读取配置文件失败: %v", err)
	}
}
