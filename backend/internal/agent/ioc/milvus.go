package ioc

import (
	"context"

	"github.com/XDWow/DouyinMall/backend/internal/agent/config"
	"github.com/XDWow/DouyinMall/backend/internal/agent/domain"
	sdkclient "github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
	"github.com/spf13/viper"
)

// InitMilvusClient 建立 Milvus 连接并确保 Collection 存在。
func InitMilvusClient() sdkclient.Client {
	c := config.MilvusConfig{
		Addr: "localhost:19530",
	}
	_ = viper.UnmarshalKey("milvus", &c)

	sdkC, err := sdkclient.NewClient(context.Background(), sdkclient.Config{
		Address: c.Addr,
	})
	if err != nil {
		panic("init milvus client: " + err.Error())
	}

	if err := ensureCollections(sdkC); err != nil {
		panic("milvus ensure collections: " + err.Error())
	}
	return sdkC
}

// ensureCollections 若 Collection 不存在则自动创建，已存在则跳过。
// agent_knowledge: id(VarChar PK) + vector(FloatVector 1024 dim, qwen3-embedding:0.6b 输出 1024 维)
// agent_semantic_cache: id(VarChar PK) + vector(FloatVector 1024 dim)
func ensureCollections(c sdkclient.Client) error {
	ctx := context.Background()
	// qwen3-embedding:0.6b 实际输出维度
	const dim = 1024

	specs := []struct {
		name string
		desc string
	}{
		{domain.CollectionKnowledge, "知识库向量集合"},
		{domain.CollectionCache, "语义缓存向量集合"},
	}

	for _, spec := range specs {
		exists, err := c.HasCollection(ctx, spec.name)
		if err != nil {
			return err
		}
		if exists {
			continue
		}

		schema := entity.NewSchema().
			WithName(spec.name).
			WithDescription(spec.desc).
			WithField(entity.NewField().
				WithName("id").
				WithDataType(entity.FieldTypeVarChar).
				WithIsPrimaryKey(true).
				WithMaxLength(64)).
			WithField(entity.NewField().
				WithName("vector").
				WithDataType(entity.FieldTypeFloatVector).
				WithDim(dim))

		if err := c.CreateCollection(ctx, schema, entity.DefaultShardNumber); err != nil {
			return err
		}

		// 建 IVF_FLAT 索引（余弦相似度）
		idx, _ := entity.NewIndexIvfFlat(entity.COSINE, 128)
		if err := c.CreateIndex(ctx, spec.name, "vector", idx, false); err != nil {
			return err
		}

		if err := c.LoadCollection(ctx, spec.name, false); err != nil {
			return err
		}
	}
	return nil
}
