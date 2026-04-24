package config

type Config struct {
	DB   DBConfig   `mapstructure:"db"`
	Etcd EtcdConfig `mapstructure:"etcd"`
	GRPC GRPCConfig `mapstructure:"grpc"`
	MCP  MCPConfig  `mapstructure:"mcp"`
}

type DBConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Database string `mapstructure:"database"`
	Params   string `mapstructure:"params"`
}

type EtcdConfig struct {
	Endpoints []string `mapstructure:"endpoints"`
}

type GRPCConfig struct {
	Server GRPCServerConfig `mapstructure:"server"`
}

type GRPCServerConfig struct {
	Name string `mapstructure:"name"`
	Port int    `mapstructure:"port"`
}

type MCPConfig struct {
	Server   ServerConfig   `mapstructure:"server"`
	Upstream UpstreamConfig `mapstructure:"upstream"`
	Tools    []ToolConfig   `mapstructure:"tools"`
}

type ServerConfig struct {
	Addr    string `mapstructure:"addr"`
	Name    string `mapstructure:"name"`
	Version string `mapstructure:"version"`
}

type UpstreamConfig struct {
	ServiceName string `mapstructure:"service_name"`
	DirectAddr  string `mapstructure:"direct_addr"`
}

type ToolConfig struct {
	Key         string `mapstructure:"key"`
	Name        string `mapstructure:"name"`
	Description string `mapstructure:"description"`
	Enabled     bool   `mapstructure:"enabled"`
}
