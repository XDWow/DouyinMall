package config

import (
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Config struct {
	DB      DBConfig      `yaml:"db"`
	Redis   RedisConfig   `yaml:"redis"`
	Etcd    EtcdConfig    `yaml:"etcd"`
	GRPC    GRPCConfig    `yaml:"grpc"`
	HTTP    HTTPConfig    `yaml:"http"`
	Payment PaymentConfig `yaml:"payment"`
	Wechat  WechatConfig  `yaml:"wechat"`
	Alipay  AlipayConfig  `yaml:"alipay"`
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
	Server GRPCServerConfig `yaml:"server"`
}

type GRPCServerConfig struct {
	Port    int `yaml:"port"`
	EtcdTTL int `yaml:"etcdTTL"`
}

type HTTPConfig struct {
	Server HTTPServerConfig `yaml:"server"`
}

type HTTPServerConfig struct {
	Port int `yaml:"port"`
}

type PaymentConfig struct {
	Mode     string `yaml:"mode"`
	Provider string `yaml:"provider"`
}

type WechatConfig struct {
	APIBaseURL     string `yaml:"api_base_url"`
	AppID          string `yaml:"app_id"`
	MchID          string `yaml:"mch_id"`
	CertSerialNo   string `yaml:"cert_serial_no"`
	PrivateKeyPath string `yaml:"private_key_path"`
	APIv3Key       string `yaml:"api_v3_key"`
	NotifyURL      string `yaml:"notify_url"`
}

type AlipayConfig struct {
	AppID      string `yaml:"app_id"`
	PID        string `yaml:"pid"`
	Gateway    string `yaml:"gateway"`
	PrivateKey string `yaml:"private_key"`
	PublicKey  string `yaml:"public_key"`
	Sandbox    bool   `yaml:"sandbox"`
	NotifyURL  string `yaml:"notify_url"`
	ReturnURL  string `yaml:"return_url"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

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
	if apiBaseURL := os.Getenv("WECHAT_API_BASE_URL"); apiBaseURL != "" {
		cfg.Wechat.APIBaseURL = apiBaseURL
	}
	if appID := os.Getenv("WECHAT_APP_ID"); appID != "" {
		cfg.Wechat.AppID = appID
	}
	if mchID := os.Getenv("WECHAT_MCH_ID"); mchID != "" {
		cfg.Wechat.MchID = mchID
	}
	if certSerialNo := os.Getenv("WECHAT_CERT_SERIAL_NO"); certSerialNo != "" {
		cfg.Wechat.CertSerialNo = certSerialNo
	}
	if privateKeyPath := os.Getenv("WECHAT_PRIVATE_KEY_PATH"); privateKeyPath != "" {
		cfg.Wechat.PrivateKeyPath = privateKeyPath
	}
	if apiV3Key := os.Getenv("WECHAT_API_V3_KEY"); apiV3Key != "" {
		cfg.Wechat.APIv3Key = apiV3Key
	}
	if notifyURL := os.Getenv("WECHAT_NOTIFY_URL"); notifyURL != "" {
		cfg.Wechat.NotifyURL = notifyURL
	}
	if appID := os.Getenv("ALIPAY_APP_ID"); appID != "" {
		cfg.Alipay.AppID = appID
	}
	if pid := os.Getenv("ALIPAY_PID"); pid != "" {
		cfg.Alipay.PID = pid
	}
	if gateway := os.Getenv("ALIPAY_GATEWAY"); gateway != "" {
		cfg.Alipay.Gateway = gateway
	}
	if privateKey := os.Getenv("ALIPAY_PRIVATE_KEY"); privateKey != "" {
		cfg.Alipay.PrivateKey = privateKey
	}
	if publicKey := os.Getenv("ALIPAY_PUBLIC_KEY"); publicKey != "" {
		cfg.Alipay.PublicKey = publicKey
	}
	if sandbox := os.Getenv("ALIPAY_SANDBOX"); sandbox != "" {
		if parsed, err := strconv.ParseBool(sandbox); err == nil {
			cfg.Alipay.Sandbox = parsed
		}
	}
	if notifyURL := os.Getenv("ALIPAY_NOTIFY_URL"); notifyURL != "" {
		cfg.Alipay.NotifyURL = notifyURL
	}
	if returnURL := os.Getenv("ALIPAY_RETURN_URL"); returnURL != "" {
		cfg.Alipay.ReturnURL = returnURL
	}
	if provider := os.Getenv("PAYMENT_PROVIDER"); provider != "" {
		cfg.Payment.Provider = provider
	}

	return &cfg, nil
}
