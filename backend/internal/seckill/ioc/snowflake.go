package ioc

import (
	"fmt"
	"hash/crc32"
	"os"

	"github.com/bwmarrin/snowflake"
	"github.com/spf13/viper"
)

type SnowflakeIDGenerator struct {
	node *snowflake.Node
}

func InitIDGenerator() *SnowflakeIDGenerator {
	nodeID := viper.GetInt64("snowflake.node_id")
	if !viper.IsSet("snowflake.node_id") {
		nodeID = defaultSnowflakeNodeID()
	}

	node, err := snowflake.NewNode(nodeID)
	if err != nil {
		panic(fmt.Errorf("init snowflake node failed: %w", err))
	}
	return &SnowflakeIDGenerator{node: node}
}

func (g *SnowflakeIDGenerator) GenerateID() string {
	return g.node.Generate().String()
}

func defaultSnowflakeNodeID() int64 {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		return 1
	}
	return int64(crc32.ChecksumIEEE([]byte(hostname)) % 1024)
}
