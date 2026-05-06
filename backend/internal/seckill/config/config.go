package config

type DBConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Database string `mapstructure:"database"`
	Params   string `mapstructure:"params"`
}

type RocketMQConfig struct {
	Endpoint               string `mapstructure:"endpoint"`
	AccessKey              string `mapstructure:"access_key"`
	SecretKey              string `mapstructure:"secret_key"`
	RequestGroup           string `mapstructure:"request_group"`
	DeadLetterGroup        string `mapstructure:"dead_letter_group"`
	InvisibleDurationSec   int    `mapstructure:"invisible_duration_sec"`
	HandleTimeoutSec       int    `mapstructure:"handle_timeout_sec"`
	ShutdownTimeoutSec     int    `mapstructure:"shutdown_timeout_sec"`
	AwaitDurationSec       int    `mapstructure:"await_duration_sec"`
	MaxMessageNum          int32  `mapstructure:"max_message_num"`
	ProducerMaxAttempts    int32  `mapstructure:"producer_max_attempts"`
	GlobalWorkerNum        int    `mapstructure:"global_worker_num"`
	PerActivityConcurrency int    `mapstructure:"per_activity_concurrency"`
}
