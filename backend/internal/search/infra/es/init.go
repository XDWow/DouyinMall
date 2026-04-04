package es

import (
	"context"
	"fmt"
)

// InitIndices 骞傜瓑鍒濆鍖栨墍鏈?ES 绱㈠紩
func InitIndices(client *ESClient) error {
	ctx := context.Background()
	if err := initIndex(ctx, client, ProductIndex, getProductIndexMapping()); err != nil {
		return fmt.Errorf("鍒濆鍖栧晢鍝佺储寮曞け璐? %w", err)
	}
	if err := initIndex(ctx, client, MerchantIndex, getMerchantIndexMapping()); err != nil {
		return fmt.Errorf("鍒濆鍖栧晢瀹剁储寮曞け璐? %w", err)
	}
	return nil
}

func initIndex(ctx context.Context, client *ESClient, name, mapping string) error {
	exists, err := client.IndexExists(ctx, name)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return client.CreateIndex(ctx, name, mapping)
}

// ES8 鍟嗗搧绱㈠紩 mapping锛屽寘鍚?dense_vector 鐢ㄤ簬鍚戦噺鎼滅储
func getProductIndexMapping() string {
	return fmt.Sprintf(`{
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
      "id":            { "type": "long" },
      "name": {
        "type": "text",
        "analyzer": "ik_max_word",
        "search_analyzer": "ik_smart",
        "fields": {
          "keyword": { "type": "keyword", "ignore_above": 256 },
          "suggest": { "type": "completion", "analyzer": "ik_smart" }
        }
      },
      "description": {
        "type": "text",
        "analyzer": "ik_max_word",
        "search_analyzer": "ik_smart"
      },
      "picture":       { "type": "keyword", "index": false },
      "slider_imgs":   { "type": "keyword", "index": false },
      "price":         { "type": "long" },
      "categories":    { "type": "keyword" },
      "in_stock":      { "type": "boolean" },
      "merchant_id":   { "type": "long" },
      "merchant_name": {
        "type": "text",
        "analyzer": "ik_smart",
        "fields": { "keyword": { "type": "keyword" } }
      },
      "sales_count":   { "type": "long" },
      "created_at":    { "type": "date", "format": "epoch_second" },
      "updated_at":    { "type": "date", "format": "epoch_second" },
      "name_vector": {
        "type": "dense_vector",
        "dims": %d,
        "index": true,
        "similarity": "cosine"
      }
    }
  }
}`, VectorDims)
}

// 鍟嗗绱㈠紩 mapping
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
      "id":            { "type": "long" },
      "name": {
        "type": "text",
        "analyzer": "ik_smart",
        "fields": {
          "keyword": { "type": "keyword" },
          "suggest": { "type": "completion", "analyzer": "ik_smart" }
        }
      },
      "description":   { "type": "text", "analyzer": "ik_smart" },
      "logo":          { "type": "keyword", "index": false },
      "region":        { "type": "keyword" },
      "rating":        { "type": "float" },
      "sales_count":   { "type": "long" },
      "product_count": { "type": "long" },
      "verified":      { "type": "boolean" },
      "created_at":    { "type": "date", "format": "epoch_second" },
      "updated_at":    { "type": "date", "format": "epoch_second" }
    }
  }
}`
}


