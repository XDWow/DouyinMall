package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	DB    DBConfig    `yaml:"db"`
	Redis RedisConfig `yaml:"redis"`
	Etcd  EtcdConfig  `yaml:"etcd"`
	GRPC  GRPCConfig  `yaml:"grpc"`
}

type DBConfig struct {
	Host     string `yaml:"host" mapstructure:"host"`
	Port     int    `yaml:"port" mapstructure:"port"`
	User     string `yaml:"user" mapstructure:"user"`
	Password string `yaml:"password" mapstructure:"password"`
	Database string `yaml:"database" mapstructure:"database"`
	Params   string `yaml:"params" mapstructure:"params"`
}

type RedisConfig struct {
	Addr     string `yaml:"addr" mapstructure:"addr"`
	Password string `yaml:"password" mapstructure:"password"`
	DB       int    `yaml:"db" mapstructure:"db"`
}

type EtcdConfig struct {
	Endpoints []string `yaml:"endpoints"`
}

type GRPCConfig struct {
	Server ServerConfig `yaml:"server"`
}

type ServerConfig struct {
	Port    int `yaml:"port"`
	EtcdTTL int `yaml:"etcdTTL"`
}

// LoadConfig 从 YAML 加载配置；环境变量可覆盖部分字段
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// 环境变量覆盖
	if password := os.Getenv("DB_PASSWORD"); password != "" {
		cfg.DB.Password = password
	}
	if redisAddr := os.Getenv("REDIS_ADDR"); redisAddr != "" {
		cfg.Redis.Addr = redisAddr
	}
	if redisPassword := os.Getenv("REDIS_PASSWORD"); redisPassword != "" {
		cfg.Redis.Password = redisPassword
	}
	if etcdEndpoints := os.Getenv("ETCD_ENDPOINTS"); etcdEndpoints != "" {
		cfg.Etcd.Endpoints = []string{etcdEndpoints}
	}

	return &cfg, nil
}
