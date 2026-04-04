package config

type Config struct {
	DB    DBConfig    `yaml:"db"`
	Redis RedisConfig `yaml:"redis"`
	Etcd  EtcdConfig  `yaml:"etcd"`
	GRPC  GRPCConfig  `yaml:"grpc"`
	Kafka KafkaConfig `yaml:"kafka"`
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
	Name string `yaml:"name"`
	Port int    `yaml:"port"`
}

type KafkaConfig struct {
	Brokers []string `yaml:"brokers"`
}


