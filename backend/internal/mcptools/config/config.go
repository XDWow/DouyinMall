package config

// MCPToolsConfig MCP 工具服务器配置
type MCPToolsConfig struct {
	Server ServerConfig `mapstructure:"server"`
	Etcd   EtcdConfig   `mapstructure:"etcd"`
}

type ServerConfig struct {
	Port int `mapstructure:"port"`
}

type EtcdConfig struct {
	Endpoints []string `mapstructure:"endpoints"`
}
