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
	NameServer          string `mapstructure:"name_server"`
	AccessKey           string `mapstructure:"access_key"`
	SecretKey           string `mapstructure:"secret_key"`
	ProducerGroup       string `mapstructure:"producer_group"`
	RequestGroup        string `mapstructure:"request_group"`
	DeadLetterGroup     string `mapstructure:"dead_letter_group"`
	OrderStatusGroup    string `mapstructure:"order_status_group"`
	HandleTimeoutSec    int    `mapstructure:"handle_timeout_sec"`
	ProducerMaxAttempts int32  `mapstructure:"producer_max_attempts"`
	GlobalWorkerNum     int    `mapstructure:"global_worker_num"`
}
