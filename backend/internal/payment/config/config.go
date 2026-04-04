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
	DSN string `yaml:"dsn"`
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
	Port    int `yaml:"port"`
	EtcdTTL int `yaml:"etcdTTL"`
}

// LoadConfig 浠庢枃浠跺姞杞介厤缃紝鐜鍙橀噺浼樺厛
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// 鐜鍙橀噺瑕嗙洊閰嶇疆鏂囦欢
	if dsn := os.Getenv("DB_DSN"); dsn != "" {
		cfg.DB.DSN = dsn
	}
	if redisAddr := os.Getenv("REDIS_ADDR"); redisAddr != "" {
		cfg.Redis.Addr = redisAddr
	}
	if etcdEndpoints := os.Getenv("ETCD_ENDPOINTS"); etcdEndpoints != "" {
		cfg.Etcd.Endpoints = []string{etcdEndpoints}
	}

	return &cfg, nil
}



