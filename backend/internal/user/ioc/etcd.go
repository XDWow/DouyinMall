package ioc

import (
	"fmt"
	"github.com/spf13/viper"
	clientv3 "go.etcd.io/etcd/client/v3"
	"time"
)

func InitEtcdClient() (*clientv3.Client, error) {
	var cfg clientv3.Config
	if err := viper.UnmarshalKey("etcd", &cfg); err != nil {
		return nil, err
	}

	if len(cfg.Endpoints) == 0 {
		return nil, fmt.Errorf("etcd endpoints is empty")
	}
	if cfg.DialTimeout == 0 {
		cfg.DialTimeout = 5 * time.Second
	}

	client, err := clientv3.New(cfg)
	if err != nil {
		return nil, err
	}
	return client, nil
}
