package config

type AgentConfig struct {
	GRPC   GRPCConfig   `mapstructure:"grpc"`
	DB     DBConfig     `mapstructure:"db"`
	Redis  RedisConfig  `mapstructure:"redis"`
	Etcd   EtcdConfig   `mapstructure:"etcd"`
	LLM    LLMConfig    `mapstructure:"llm"`
	Embed  EmbedConfig  `mapstructure:"embedding"`
	Milvus MilvusConfig `mapstructure:"milvus"`
	Kafka  KafkaConfig  `mapstructure:"kafka"`
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
}

type EmbedConfig struct {
	BaseURL string `mapstructure:"base_url"`
	Model   string `mapstructure:"model"`
	Timeout int    `mapstructure:"timeout_seconds"`
}

type MilvusConfig struct {
	Addr string `mapstructure:"addr"`
}

type KafkaConfig struct {
	// Brokers Kafka broker 地址列表，支持单字符串或字符串数组
	Brokers []string `mapstructure:"brokers"`
	// ProducerRetryMax 生产者发送失败最大重试次数，默认 3
	ProducerRetryMax int `mapstructure:"producer_retry_max"`
	// ConsumerOffsetsInitial 消费者起始 offset："newest"（默认）或 "oldest"
	ConsumerOffsetsInitial string `mapstructure:"consumer_offsets_initial"`
}
