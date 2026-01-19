package es

import (
	"context"
	"fmt"
)

// InitIndices 初始化 ES 索引（类似数据库表的 AutoMigrate）
func InitIndices(client *ESClient) error {
	ctx := context.Background()
	// 初始化商品索引
	if err := initProductIndex(ctx, client); err != nil {
		return fmt.Errorf("初始化商品索引失败: %w", err)
	}

	// 初始化商家索引
	if err := initMerchantIndex(ctx, client); err != nil {
		return fmt.Errorf("初始化商家索引失败: %w", err)
	}

	return nil
}

func initProductIndex(ctx context.Context, client *ESClient) error {
	indexName := ProductIndex

	// 检查索引是否已存在（保证幂等性）
	exists, err := client.IndexExists(ctx, indexName)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	// 创建索引
	mapping := getProductIndexMapping()
	if err := client.CreateIndex(ctx, indexName, mapping); err != nil {
		return err
	}

	return nil
}

func initMerchantIndex(ctx context.Context, client *ESClient) error {
	indexName := MerchantIndex

	// 检查索引是否已存在（保证幂等性）
	exists, err := client.IndexExists(ctx, indexName)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	// 创建索引
	mapping := getMerchantIndexMapping()
	if err := client.CreateIndex(ctx, indexName, mapping); err != nil {
		return err
	}

	return nil
}

// 商品索引的 mapping 配置
// 注意：IK 分词器的自定义词典需要在 ES 服务器端配置
// 1. 将 config/ik-dict/product.dic 复制到 ES 的 config/analysis-ik/ 目录
// 2. 在 ES 的 config/analysis-ik/IKAnalyzer.cfg.xml 中配置 ext_dict 或 remote_ext_dict
// 3. 重启 ES 服务（本地词典）或等待自动加载（远程词典）
// 详细部署说明请参考：docs/IK_DICT_DEPLOY.md
func getProductIndexMapping() string {
	return `{
  "settings": {
    "number_of_shards": 3,
    "number_of_replicas": 1,
    "analysis": {
      "analyzer": {
        "ik_smart_analyzer": {
          "type": "custom",
          "tokenizer": "ik_smart",
          "filter": ["lowercase"]
        },
        "ik_max_word_analyzer": {
          "type": "custom",
          "tokenizer": "ik_max_word",
          "filter": ["lowercase"]
        }
      }
    }
  },
  "mappings": {
    "properties": {
      "id": {
        "type": "long"
      },
      "name": {
        "type": "text",
        "analyzer": "ik_max_word",
        "search_analyzer": "ik_smart",
        "fields": {
          "keyword": {
            "type": "keyword",
            "ignore_above": 256
          },
          "suggest": {
            "type": "completion",
            "analyzer": "ik_smart"
          }
        }
      },
      "description": {
        "type": "text",
        "analyzer": "ik_max_word",
        "search_analyzer": "ik_smart"
      },
      "picture": {
        "type": "keyword",
        "index": false
      },
      "slider_imgs": {
        "type": "keyword",
        "index": false
      },
      "price": {
        "type": "long"
      },
      "categories": {
        "type": "keyword"
      },
      "in_stock": {
        "type": "boolean"
      },
      "merchant_id": {
        "type": "long"
      },
      "merchant_name": {
        "type": "text",
        "analyzer": "ik_smart",
        "fields": {
          "keyword": {
            "type": "keyword"
          }
        }
      },
      "created_at": {
        "type": "date",
        "format": "epoch_second"
      },
      "updated_at": {
        "type": "date",
        "format": "epoch_second"
      }
    }
  }
}`
}

// 商家索引的 mapping 配置
func getMerchantIndexMapping() string {
	return `{
  "settings": {
    "number_of_shards": 2,
    "number_of_replicas": 1,
    "analysis": {
      "analyzer": {
        "ik_smart_analyzer": {
          "type": "custom",
          "tokenizer": "ik_smart"
        }
      }
    }
  },
  "mappings": {
    "properties": {
      "id": {
        "type": "long"
      },
      "name": {
        "type": "text",
        "analyzer": "ik_smart",
        "fields": {
          "keyword": {
            "type": "keyword"
          }
        }
      },
      "description": {
        "type": "text",
        "analyzer": "ik_smart"
      },
      "logo": {
        "type": "keyword",
        "index": false
      },
      "region": {
        "type": "keyword"
      },
      "rating": {
        "type": "float"
      },
      "sales_count": {
        "type": "long"
      },
      "product_count": {
        "type": "long"
      },
      "verified": {
        "type": "boolean"
      },
      "created_at": {
        "type": "date",
        "format": "epoch_second"
      },
      "updated_at": {
        "type": "date",
        "format": "epoch_second"
      }
    }
  }
}`
}
