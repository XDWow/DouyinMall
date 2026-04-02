package main

import (
	"context"
	"fmt"
	"log"
	"os/signal"
	"strings"
	"syscall"
	"time"

	agentconfig "github.com/XDWow/DouyinMall/backend/internal/agent/config"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func main() {
	cfg := initConfig()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	app, err := NewApp(ctx, cfg)
	if err != nil {
		log.Fatalf("init agent app failed: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- app.Start()
	}()

	select {
	case err = <-errCh:
		if err != nil {
			log.Fatalf("agent server stopped with error: %v", err)
		}
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := app.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("shutdown agent app failed: %v", err)
	}
}

func initConfig() agentconfig.Config {
	configPath := pflag.String("config", "internal/agent/config/dev.yaml", "agent config file path")
	pflag.Parse()

	viper.SetConfigFile(*configPath)
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()
	bindEnv()

	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("read config file failed: %w", err))
	}

	var cfg agentconfig.Config
	if err := viper.Unmarshal(&cfg); err != nil {
		panic(fmt.Errorf("unmarshal config failed: %w", err))
	}
	return cfg
}

func bindEnv() {
	mustBindEnv("http.addr", "HTTP_ADDR")
	mustBindEnv("grpc.server.port", "GRPC_PORT")
	mustBindEnv("grpc.server.name", "GRPC_SERVICE_NAME")
	mustBindEnv("db.dsn", "DB_DSN")
	mustBindEnv("redis.addr", "REDIS_ADDR")
	mustBindEnv("redis.password", "REDIS_PASSWORD")
	mustBindEnv("redis.db", "REDIS_DB")
	mustBindEnv("etcd.endpoints", "ETCD_ENDPOINTS")
	mustBindEnv("llm.base_url", "LLM_BASE_URL")
	mustBindEnv("llm.api_key", "LLM_API_KEY")
	mustBindEnv("llm.model", "LLM_MODEL")
	mustBindEnv("embedding.base_url", "EMBEDDING_BASE_URL")
	mustBindEnv("embedding.api_key", "EMBEDDING_API_KEY")
	mustBindEnv("embedding.model", "EMBEDDING_MODEL")
	mustBindEnv("observability.trace.enabled", "OTEL_TRACE_ENABLED")
	mustBindEnv("observability.trace.endpoint", "OTEL_EXPORTER_OTLP_ENDPOINT")
	mustBindEnv("observability.trace.service_name", "OTEL_SERVICE_NAME")
}

func mustBindEnv(key string, envs ...string) {
	args := append([]string{key}, envs...)
	if err := viper.BindEnv(args...); err != nil {
		panic(fmt.Errorf("bind env for %s failed: %w", key, err))
	}
}
