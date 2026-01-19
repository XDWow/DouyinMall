package es

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"
)

const (
	ProductIndex  = "product_index"
	MerchantIndex = "merchant_index"
)

type ESClient struct {
	client *elasticsearch.Client
}

func NewESClient(addresses []string) (*ESClient, error) {
	cfg := elasticsearch.Config{
		Addresses: addresses,
	}

	client, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("创建 ES 客户端失败: %w", err)
	}

	return &ESClient{client: client}, nil
}

func (c *ESClient) Search(ctx context.Context, indexName string, query string) (*esapi.Response, error) {
	res, err := c.client.Search(
		c.client.Search.WithContext(ctx),
		c.client.Search.WithIndex(indexName),
		c.client.Search.WithBody(strings.NewReader(query)),
		c.client.Search.WithTrackTotalHits(true),
	)
	if err != nil {
		return nil, fmt.Errorf("搜索失败: %w", err)
	}
	return res, nil
}

func (c *ESClient) SearchSuggest(ctx context.Context, indexName string, keyword string, limit int64, needCount bool) ([]map[string]interface{}, error) {
	// needCount: 是否需要查询匹配数量（商品需要，商家不需要）
	query := map[string]interface{}{
		"suggest": map[string]interface{}{
			"product_suggest": map[string]interface{}{
				"prefix": keyword,
				"completion": map[string]interface{}{
					"field": "name.suggest",
					"size":  limit,
				},
			},
		},
	}

	queryJSON, _ := json.Marshal(query)
	res, err := c.client.Search(
		c.client.Search.WithContext(ctx),
		c.client.Search.WithIndex(indexName),
		c.client.Search.WithBody(bytes.NewReader(queryJSON)),
	)
	if err != nil {
		return nil, fmt.Errorf("搜索建议失败: %w", err)
	}
	defer res.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析建议结果失败: %w", err)
	}

	suggestions := []map[string]interface{}{}
	if suggest, ok := result["suggest"].(map[string]interface{}); ok {
		if productSuggest, ok := suggest["product_suggest"].([]interface{}); ok && len(productSuggest) > 0 {
			if options, ok := productSuggest[0].(map[string]interface{})["options"].([]interface{}); ok {
				for _, opt := range options {
					if optMap, ok := opt.(map[string]interface{}); ok {
						suggestions = append(suggestions, optMap)
					}
				}
			}
		}
	}

	return suggestions, nil
}

func (c *ESClient) IndexDocument(ctx context.Context, indexName string, docID string, doc interface{}) error {
	docJSON, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("序列化文档失败: %w", err)
	}

	req := esapi.IndexRequest{
		Index:      indexName,
		DocumentID: docID,
		Body:       bytes.NewReader(docJSON),
		Refresh:    "true",
	}

	res, err := req.Do(ctx, c.client)
	if err != nil {
		return fmt.Errorf("索引文档失败: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("索引文档失败: %s", string(body))
	}

	return nil
}

func (c *ESClient) UpdateDocument(ctx context.Context, indexName string, docID string, doc interface{}) error {
	docJSON, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("序列化文档失败: %w", err)
	}

	req := esapi.UpdateRequest{
		Index:      indexName,
		DocumentID: docID,
		Body:       bytes.NewReader([]byte(fmt.Sprintf(`{"doc":%s}`, string(docJSON)))),
		Refresh:    "true",
	}

	res, err := req.Do(ctx, c.client)
	if err != nil {
		return fmt.Errorf("更新文档失败: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("更新文档失败: %s", string(body))
	}

	return nil
}

func (c *ESClient) DeleteDocument(ctx context.Context, indexName string, docID string) error {
	req := esapi.DeleteRequest{
		Index:      indexName,
		DocumentID: docID,
		Refresh:    "true",
	}

	res, err := req.Do(ctx, c.client)
	if err != nil {
		return fmt.Errorf("删除文档失败: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("删除文档失败: %s", string(body))
	}

	return nil
}

func (c *ESClient) BulkIndex(ctx context.Context, indexName string, docs []map[string]interface{}) error {
	var body bytes.Buffer
	for _, doc := range docs {
		// 元数据行
		meta := map[string]interface{}{
			"index": map[string]interface{}{
				"_index": indexName,
				"_id":    doc["id"],
			},
		}
		metaJSON, _ := json.Marshal(meta)
		body.Write(metaJSON)
		body.WriteByte('\n')

		// 文档行
		docJSON, _ := json.Marshal(doc)
		body.Write(docJSON)
		body.WriteByte('\n')
	}

	req := esapi.BulkRequest{
		Body:    bytes.NewReader(body.Bytes()),
		Refresh: "true",
	}

	res, err := req.Do(ctx, c.client)
	if err != nil {
		return fmt.Errorf("批量索引失败: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("批量索引失败: %s", string(body))
	}

	return nil
}

func (c *ESClient) BulkDelete(ctx context.Context, indexName string, docIDs []string) error {
	var body bytes.Buffer
	for _, docID := range docIDs {
		meta := map[string]interface{}{
			"delete": map[string]interface{}{
				"_index": indexName,
				"_id":    docID,
			},
		}
		metaJSON, _ := json.Marshal(meta)
		body.Write(metaJSON)
		body.WriteByte('\n')
	}

	req := esapi.BulkRequest{
		Body:    bytes.NewReader(body.Bytes()),
		Refresh: "true",
	}

	res, err := req.Do(ctx, c.client)
	if err != nil {
		return fmt.Errorf("批量删除失败: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("批量删除失败: %s", string(body))
	}

	return nil
}

func (c *ESClient) IndexExists(ctx context.Context, indexName string) (bool, error) {
	req := esapi.IndicesExistsRequest{
		Index: []string{indexName},
	}

	res, err := req.Do(ctx, c.client)
	if err != nil {
		return false, fmt.Errorf("检查索引是否存在失败: %w", err)
	}
	defer res.Body.Close()

	return res.StatusCode == 200, nil
}

func (c *ESClient) CreateIndex(ctx context.Context, indexName string, mapping string) error {
	req := esapi.IndicesCreateRequest{
		Index: indexName,
		Body:  strings.NewReader(mapping),
	}

	res, err := req.Do(ctx, c.client)
	if err != nil {
		return fmt.Errorf("创建索引失败: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("创建索引失败: %s", string(body))
	}

	return nil
}
