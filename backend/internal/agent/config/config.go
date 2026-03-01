package config

// AgentConfig Agent 服务配置
type AgentConfig struct {
	GRPC   GRPCConfig   `mapstructure:"grpc"`
	DB     DBConfig     `mapstructure:"db"`
	Redis  RedisConfig  `mapstructure:"redis"`
	Etcd   EtcdConfig   `mapstructure:"etcd"`
	LLM    LLMConfig    `mapstructure:"llm"`
	Embed  EmbedConfig  `mapstructure:"embedding"`
	Milvus MilvusConfig `mapstructure:"milvus"`
}

type GRPCConfig struct {
	Port int    `mapstructure:"port"`
	Name string `mapstructure:"name"`
}

type DBConfig struct {
	DSN string `mapstructure:"dsn"`
}

type RedisConfig struct {
	Addr string `mapstructure:"addr"`
}

type EtcdConfig struct {
	Endpoints []string `mapstructure:"endpoints"`
}

type LLMConfig struct {
	BaseURL string `mapstructure:"base_url"`
	APIKey  string `mapstructure:"api_key"`
	Model   string `mapstructure:"chat_model"`
	Timeout int    `mapstructure:"timeout_seconds"`
}

type EmbedConfig struct {
	BaseURL string `mapstructure:"base_url"`
	APIKey  string `mapstructure:"api_key"`
	Model   string `mapstructure:"model"`
	Timeout int    `mapstructure:"timeout_seconds"`
}

type MilvusConfig struct {
	Addr       string `mapstructure:"addr"`
	Collection string `mapstructure:"collection"`
}
