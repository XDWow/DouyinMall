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
	"github.com/joho/godotenv"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func main() {
	// Load optional .env from current working directory (e.g. backend/.env); ignore if missing.
	_ = godotenv.Load()

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
	bindSecretsFromEnv()

	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("read config file failed: %w", err))
	}

	var cfg agentconfig.Config
	if err := viper.Unmarshal(&cfg); err != nil {
		panic(fmt.Errorf("unmarshal config failed: %w", err))
	}
	return cfg
}

// bindSecretsFromEnv 仅从环境变量注入密钥类配置；其余依赖配置文件。
func bindSecretsFromEnv() {
	mustBindEnv("db.dsn", "DB_DSN")
	mustBindEnv("llm.weak.api_key", "LLM_WEAK_API_KEY", "LLM_API_KEY")
	mustBindEnv("llm.strong.api_key", "LLM_STRONG_API_KEY", "LLM_API_KEY")
	mustBindEnv("redis.password", "REDIS_PASSWORD")
	mustBindEnv("embedding.api_key", "EMBEDDING_API_KEY", "LLM_API_KEY")
	mustBindEnv("knowledge_base.qdrant.host", "QDRANT_HOST")
	mustBindEnv("knowledge_base.qdrant.port", "QDRANT_PORT")
	mustBindEnv("knowledge_base.qdrant.api_key", "QDRANT_API_KEY")
	mustBindEnv("knowledge_base.qdrant.collection", "QDRANT_COLLECTION")
	mustBindEnv("knowledge_base.qdrant.use_tls", "QDRANT_USE_TLS")
}

func mustBindEnv(key string, envs ...string) {
	args := append([]string{key}, envs...)
	if err := viper.BindEnv(args...); err != nil {
		panic(fmt.Errorf("bind env for %s failed: %w", key, err))
	}
}
