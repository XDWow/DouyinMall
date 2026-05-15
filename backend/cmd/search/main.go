package main

import (
	"fmt"
	"log"

	"github.com/XDWow/DouyinMall/backend/pkg/envx"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func main() {
	initViperWatch()

	app := InitApp()

	log.Printf("search service starting, grpc port=%d http port=%d",
		viper.GetInt("grpc.server.port"),
		viper.GetInt("http.server.port"))

	for i, consumer := range app.Consumers {
		if err := consumer.Start(); err != nil {
			log.Fatalf("consumer %d start failed: %v", i+1, err)
		}
		log.Printf("consumer %d started", i+1)
	}

	grpcErr := make(chan error, 1)
	go func() {
		grpcErr <- app.Server.Run()
	}()

	httpErr := make(chan error, 1)
	go func() {
		httpErr <- app.HTTPServer.Start()
	}()

	select {
	case err := <-grpcErr:
		if err != nil {
			log.Fatalf("grpc server run error: %v", err)
		}
	case err := <-httpErr:
		if err != nil {
			log.Fatalf("http server run error: %v", err)
		}
	}
}

func initViperWatch() {
	if err := envx.Load(); err != nil {
		panic(fmt.Errorf("load .env failed: %w", err))
	}

	cfile := pflag.String("config", "internal/search/config/dev.yaml", "config file path")
	pflag.Parse()
	viper.SetConfigFile(*cfile)
	viper.WatchConfig()
	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("read config failed: %w", err))
	}

	viper.AutomaticEnv()
	_ = viper.BindEnv("db.password", "DB_PASSWORD")
	_ = viper.BindEnv("llm.api_key", "SEARCH_LLM_API_KEY", "LLM_API_KEY")
	_ = viper.BindEnv("embedding.api_key", "SEARCH_EMBEDDING_API_KEY", "EMBEDDING_API_KEY")
	_ = viper.BindEnv("elasticsearch.addresses", "ES_ADDRESSES")
	_ = viper.BindEnv("kafka.brokers", "KAFKA_BROKERS")
	_ = viper.BindEnv("etcd.endpoints", "ETCD_ENDPOINTS")
	_ = viper.BindEnv("grpc.server.port", "GRPC_PORT")
	_ = viper.BindEnv("grpc.server.name", "GRPC_SERVICE_NAME")
}
