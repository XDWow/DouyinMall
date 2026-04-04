package main

import (
	"context"
	"fmt"
	"log"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func main() {
	initViperWatch()

	app := InitApp()

	port := viper.GetInt("grpc.server.port")
	log.Printf("Product service starting on port %d...", port)

	if viper.GetBool("canal.enabled") {
		ctx := context.Background()
		for i, p := range app.Producers {
			producer := p
			go func(idx int) {
				if err := producer.Start(ctx); err != nil {
					log.Fatalf("producer %d start failed: %v", idx+1, err)
				}
			}(i)
			log.Printf("Producer %d started", i+1)
		}
	} else {
		log.Printf("Canal producer disabled by config")
	}

	if err := app.Server.Run(); err != nil {
		log.Fatalf("server run error: %v", err)
	}
}

func initViperWatch() {
	cfile := pflag.String("config", "internal/product/config/dev.yaml", "config file path")
	pflag.Parse()
	viper.SetConfigFile(*cfile)
	viper.WatchConfig()
	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("read config failed: %w", err))
	}

	viper.AutomaticEnv()
	_ = viper.BindEnv("db.dsn", "DB_DSN")
	_ = viper.BindEnv("redis.addr", "REDIS_ADDR")
	_ = viper.BindEnv("etcd.endpoints", "ETCD_ENDPOINTS")
	_ = viper.BindEnv("kafka.brokers", "KAFKA_BROKERS")
	_ = viper.BindEnv("canal.enabled", "CANAL_ENABLED")
}


