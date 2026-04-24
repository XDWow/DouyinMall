package config

type DBConfig struct {
	Host     string `yaml:"host" mapstructure:"host"`
	Port     int    `yaml:"port" mapstructure:"port"`
	User     string `yaml:"user" mapstructure:"user"`
	Password string `yaml:"password" mapstructure:"password"`
	Database string `yaml:"database" mapstructure:"database"`
	Params   string `yaml:"params" mapstructure:"params"`
}

type RedisConfig struct {
	Addr string `yaml:"addr"`
}

type EtcdConfig struct {
	Endpoints []string `yaml:"endpoints"`
}

type GRPCConfig struct {
	Server ServerConfig `yaml:"server"`
}

type ServerConfig struct {
	Name string `yaml:"name"`
	Port int    `yaml:"port"`
}
