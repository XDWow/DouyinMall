package config

// Config checkout 服务配置（无状态服务，不需要 DB/Redis）
type Config struct {
	Etcd      EtcdConfig      `yaml:"etcd"`
	GRPC      GRPCConfig      `yaml:"grpc"`
	Snowflake SnowflakeConfig `yaml:"snowflake"`
}

type EtcdConfig struct {
	Endpoints []string `yaml:"endpoints"`
}

type GRPCConfig struct {
	Server ServerConfig `yaml:"server"`
}

type ServerConfig struct {
	Port int    `yaml:"port"`
	Name string `yaml:"name"`
}

type SnowflakeConfig struct {
	NodeID int64 `yaml:"node_id"`
}
