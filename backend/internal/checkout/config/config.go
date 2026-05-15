package config

// Config defines checkout service configuration. Checkout is a stateless
// orchestration service, so it does not need its own DB or Redis section here.
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
