// knowledge-init 是知识库初始化的独立入口。
// 执行一次性的分片 → embedding → 写入 Milvus/MySQL/Redis 流程后退出。
//
// 用法:
//
//	go run ./cmd/knowledge-init [--config path/to/config.yaml]
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/XDWow/DouyinMall/backend/internal/agent/infra/cache"
	"github.com/XDWow/DouyinMall/backend/internal/agent/ioc"
	"github.com/XDWow/DouyinMall/backend/internal/agent/knowledge"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func main() {
	pflag.String("config", "cmd/knowledge-init/host.yaml", "配置文件路径")
	pflag.Parse()

	viper.SetConfigFile(pflag.Lookup("config").Value.String())
	if err := viper.ReadInConfig(); err != nil {
		fmt.Printf("读取配置文件失败: %v\n", err)
		os.Exit(1)
	}
	viper.AutomaticEnv()
	viper.BindEnv("llm.api_key", "LLM_API_KEY")
	viper.BindEnv("reranker.api_key", "SILICONFLOW_API_KEY")

	indexer := newIndexer()
	if err := indexer.Run(context.Background()); err != nil {
		fmt.Printf("知识库初始化失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("知识库初始化完成")
}

func newIndexer() *knowledge.Indexer {
	logger := ioc.InitLogger()
	db := ioc.InitDB()
	embedder := ioc.InitEmbedder()
	milvus := ioc.InitMilvusClient()
	redis := ioc.InitRedis()
	agentRedis := cache.NewAgentRedis(redis)
	return knowledge.NewIndexer(embedder, milvus, db, agentRedis, logger)
}
