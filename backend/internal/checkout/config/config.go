package config

// Config checkout 鏈嶅姟閰嶇疆锛堟棤鐘舵€佹湇鍔★紝涓嶉渶瑕?DB/Redis锛?
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


