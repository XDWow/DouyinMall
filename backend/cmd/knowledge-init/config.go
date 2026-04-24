package main

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	DB        DBConfig        `mapstructure:"db"`
	Embedding EmbeddingConfig `mapstructure:"embedding"`
	Ingest    IngestConfig    `mapstructure:"ingest"`
	Qdrant    QdrantConfig    `mapstructure:"qdrant"`
}

type DBConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Database string `mapstructure:"database"`
	Params   string `mapstructure:"params"`
}

type EmbeddingConfig struct {
	Provider       string `mapstructure:"provider"`
	BaseURL        string `mapstructure:"base_url"`
	APIKey         string `mapstructure:"api_key"`
	Model          string `mapstructure:"model"`
	TimeoutSeconds int    `mapstructure:"timeout_seconds"`
}

type IngestConfig struct {
	ChunkSize          int    `mapstructure:"chunk_size"`
	OverlapSize        int    `mapstructure:"overlap_size"`
	HTTPTimeoutSeconds int    `mapstructure:"http_timeout_seconds"`
	DefaultCategory    string `mapstructure:"default_category"`
	Selector           string `mapstructure:"selector"`
	JinritemaiGraphID  int    `mapstructure:"jinritemai_graph_id"`
}

type QdrantConfig struct {
	Host       string `mapstructure:"host"`
	Port       int    `mapstructure:"port"`
	APIKey     string `mapstructure:"api_key"`
	Collection string `mapstructure:"collection"`
	UseTLS     bool   `mapstructure:"use_tls"`
	VectorDim  int    `mapstructure:"vector_dim"`
	BatchSize  int    `mapstructure:"batch_size"`
	Distance   string `mapstructure:"distance"`
}

func initConfig(configPath string, mode string) Config {
	viper.SetConfigFile(configPath)
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	bindEnv()

	if err := viper.ReadInConfig(); err != nil {
		logFatal("read config file failed: %v", err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		logFatal("unmarshal config failed: %v", err)
	}

	applyDefaults(&cfg)
	validateConfig(cfg, mode)
	return cfg
}

func applyDefaults(cfg *Config) {
	if strings.TrimSpace(cfg.Embedding.Provider) == "" {
		cfg.Embedding.Provider = "ollama"
	}
	if cfg.Embedding.TimeoutSeconds <= 0 {
		cfg.Embedding.TimeoutSeconds = 15
	}
	if cfg.Ingest.ChunkSize <= 0 {
		cfg.Ingest.ChunkSize = 800
	}
	if cfg.Ingest.OverlapSize < 0 {
		cfg.Ingest.OverlapSize = 0
	}
	if cfg.Ingest.OverlapSize == 0 {
		cfg.Ingest.OverlapSize = 120
	}
	if cfg.Ingest.HTTPTimeoutSeconds <= 0 {
		cfg.Ingest.HTTPTimeoutSeconds = 30
	}
	if strings.TrimSpace(cfg.Ingest.DefaultCategory) == "" {
		cfg.Ingest.DefaultCategory = "policy"
	}
	if cfg.Ingest.JinritemaiGraphID <= 0 {
		cfg.Ingest.JinritemaiGraphID = 312
	}
	if strings.TrimSpace(cfg.Qdrant.Host) == "" {
		cfg.Qdrant.Host = "127.0.0.1"
	}
	if cfg.Qdrant.Port <= 0 {
		cfg.Qdrant.Port = 6334
	}
	if strings.TrimSpace(cfg.Qdrant.Collection) == "" {
		cfg.Qdrant.Collection = "agent_knowledge"
	}
	if cfg.Qdrant.BatchSize <= 0 {
		cfg.Qdrant.BatchSize = 16
	}
	if strings.TrimSpace(cfg.Qdrant.Distance) == "" {
		cfg.Qdrant.Distance = "cosine"
	}
}

func validateConfig(cfg Config, mode string) {
	switch mode {
	case modeStore, modeFull:
		if strings.TrimSpace(cfg.Embedding.BaseURL) == "" {
			logFatal("embedding.base_url is required in %s mode", mode)
		}
		if strings.TrimSpace(cfg.Embedding.Model) == "" {
			logFatal("embedding.model is required in %s mode", mode)
		}
		if strings.TrimSpace(cfg.Qdrant.Collection) == "" {
			logFatal("qdrant.collection is required in %s mode", mode)
		}
	}
}

func bindEnv() {
	mustBindEnv("db.password", "DB_PASSWORD")
	mustBindEnv("embedding.provider", "EMBEDDING_PROVIDER")
	mustBindEnv("embedding.base_url", "EMBEDDING_BASE_URL")
	mustBindEnv("embedding.api_key", "EMBEDDING_API_KEY", "LLM_API_KEY")
	mustBindEnv("embedding.model", "EMBEDDING_MODEL")
	mustBindEnv("qdrant.host", "QDRANT_HOST")
	mustBindEnv("qdrant.port", "QDRANT_PORT")
	mustBindEnv("qdrant.api_key", "QDRANT_API_KEY")
	mustBindEnv("qdrant.collection", "QDRANT_COLLECTION")
	mustBindEnv("qdrant.use_tls", "QDRANT_USE_TLS")
}

func mustBindEnv(key string, envs ...string) {
	args := append([]string{key}, envs...)
	if err := viper.BindEnv(args...); err != nil {
		logFatal("bind env for %s failed: %v", key, err)
	}
}

func logFatal(format string, args ...any) {
	panic(fmt.Errorf(format, args...))
}
