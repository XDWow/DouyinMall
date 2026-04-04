package es

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

const (
	ProductIndex  = "product_index"
	MerchantIndex = "merchant_index"

	// 鍚戦噺缁村害锛岄渶涓?Embedding 妯″瀷杈撳嚭缁村害涓€鑷?
	VectorDims = 1024
)

type ESClient struct {
	client *elasticsearch.Client
}

func NewESClient(addresses []string) (*ESClient, error) {
	cfg := elasticsearch.Config{Addresses: addresses}
	client, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("鍒涘缓 ES 瀹㈡埛绔け璐? %w", err)
	}
	return &ESClient{client: client}, nil
}

// Search 鎵ц鑷畾涔?JSON 鏌ヨ
func (c *ESClient) Search(ctx context.Context, index string, query string) (*esapi.Response, error) {
	res, err := c.client.Search(
		c.client.Search.WithContext(ctx),
		c.client.Search.WithIndex(index),
		c.client.Search.WithBody(strings.NewReader(query)),
		c.client.Search.WithTrackTotalHits(true),
	)
	if err != nil {
		return nil, fmt.Errorf("鎼滅储澶辫触: %w", err)
	}
	return res, nil
}

// KnnSearch 鎵ц ES8 鍘熺敓 kNN 鎼滅储
func (c *ESClient) KnnSearch(ctx context.Context, index string, query string) (*esapi.Response, error) {
	res, err := c.client.Search(
		c.client.Search.WithContext(ctx),
		c.client.Search.WithIndex(index),
		c.client.Search.WithBody(strings.NewReader(query)),
		c.client.Search.WithTrackTotalHits(true),
	)
	if err != nil {
		return nil, fmt.Errorf("kNN 鎼滅储澶辫触: %w", err)
	}
	return res, nil
}

// SearchSuggest 鑷姩琛ュ叏寤鸿
func (c *ESClient) SearchSuggest(ctx context.Context, index string, keyword string, limit int64) ([]map[string]interface{}, error) {
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
		c.client.Search.WithIndex(index),
		c.client.Search.WithBody(bytes.NewReader(queryJSON)),
	)
	if err != nil {
		return nil, fmt.Errorf("鎼滅储寤鸿澶辫触: %w", err)
	}
	defer res.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("瑙ｆ瀽寤鸿缁撴灉澶辫触: %w", err)
	}

	var suggestions []map[string]interface{}
	if suggest, ok := result["suggest"].(map[string]interface{}); ok {
		if ps, ok := suggest["product_suggest"].([]interface{}); ok && len(ps) > 0 {
			if options, ok := ps[0].(map[string]interface{})["options"].([]interface{}); ok {
				for _, opt := range options {
					if m, ok := opt.(map[string]interface{}); ok {
						suggestions = append(suggestions, m)
					}
				}
			}
		}
	}
	return suggestions, nil
}

// IndexDocument 绱㈠紩鍗曚釜鏂囨。
func (c *ESClient) IndexDocument(ctx context.Context, index, docID string, doc interface{}) error {
	docJSON, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("搴忓垪鍖栨枃妗ｅけ璐? %w", err)
	}
	req := esapi.IndexRequest{
		Index:      index,
		DocumentID: docID,
		Body:       bytes.NewReader(docJSON),
		Refresh:    "true",
	}
	res, err := req.Do(ctx, c.client)
	if err != nil {
		return fmt.Errorf("绱㈠紩鏂囨。澶辫触: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("绱㈠紩鏂囨。澶辫触: %s", body)
	}
	return nil
}

// UpdateDocument 鏇存柊鏂囨。
func (c *ESClient) UpdateDocument(ctx context.Context, index, docID string, doc interface{}) error {
	docJSON, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("搴忓垪鍖栨枃妗ｅけ璐? %w", err)
	}
	req := esapi.UpdateRequest{
		Index:      index,
		DocumentID: docID,
		Body:       bytes.NewReader([]byte(fmt.Sprintf(`{"doc":%s}`, docJSON))),
		Refresh:    "true",
	}
	res, err := req.Do(ctx, c.client)
	if err != nil {
		return fmt.Errorf("鏇存柊鏂囨。澶辫触: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("鏇存柊鏂囨。澶辫触: %s", body)
	}
	return nil
}

// DeleteDocument 鍒犻櫎鏂囨。
func (c *ESClient) DeleteDocument(ctx context.Context, index, docID string) error {
	req := esapi.DeleteRequest{
		Index:      index,
		DocumentID: docID,
		Refresh:    "true",
	}
	res, err := req.Do(ctx, c.client)
	if err != nil {
		return fmt.Errorf("鍒犻櫎鏂囨。澶辫触: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("鍒犻櫎鏂囨。澶辫触: %s", body)
	}
	return nil
}

// BulkIndex 鎵归噺绱㈠紩
func (c *ESClient) BulkIndex(ctx context.Context, index string, docs []map[string]interface{}) error {
	var body bytes.Buffer
	for _, doc := range docs {
		meta := map[string]interface{}{
			"index": map[string]interface{}{"_index": index, "_id": doc["id"]},
		}
		metaJSON, _ := json.Marshal(meta)
		body.Write(metaJSON)
		body.WriteByte('\n')
		docJSON, _ := json.Marshal(doc)
		body.Write(docJSON)
		body.WriteByte('\n')
	}
	req := esapi.BulkRequest{Body: bytes.NewReader(body.Bytes()), Refresh: "true"}
	res, err := req.Do(ctx, c.client)
	if err != nil {
		return fmt.Errorf("鎵归噺绱㈠紩澶辫触: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		b, _ := io.ReadAll(res.Body)
		return fmt.Errorf("鎵归噺绱㈠紩澶辫触: %s", b)
	}
	return nil
}

// BulkDelete 鎵归噺鍒犻櫎
func (c *ESClient) BulkDelete(ctx context.Context, index string, docIDs []string) error {
	var body bytes.Buffer
	for _, id := range docIDs {
		meta := map[string]interface{}{
			"delete": map[string]interface{}{"_index": index, "_id": id},
		}
		metaJSON, _ := json.Marshal(meta)
		body.Write(metaJSON)
		body.WriteByte('\n')
	}
	req := esapi.BulkRequest{Body: bytes.NewReader(body.Bytes()), Refresh: "true"}
	res, err := req.Do(ctx, c.client)
	if err != nil {
		return fmt.Errorf("鎵归噺鍒犻櫎澶辫触: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		b, _ := io.ReadAll(res.Body)
		return fmt.Errorf("鎵归噺鍒犻櫎澶辫触: %s", b)
	}
	return nil
}

// IndexExists 妫€鏌ョ储寮曟槸鍚﹀瓨鍦?
func (c *ESClient) IndexExists(ctx context.Context, index string) (bool, error) {
	req := esapi.IndicesExistsRequest{Index: []string{index}}
	res, err := req.Do(ctx, c.client)
	if err != nil {
		return false, fmt.Errorf("妫€鏌ョ储寮曞け璐? %w", err)
	}
	defer res.Body.Close()
	return res.StatusCode == 200, nil
}

// CreateIndex 鍒涘缓绱㈠紩
func (c *ESClient) CreateIndex(ctx context.Context, index, mapping string) error {
	req := esapi.IndicesCreateRequest{
		Index: index,
		Body:  strings.NewReader(mapping),
	}
	res, err := req.Do(ctx, c.client)
	if err != nil {
		return fmt.Errorf("鍒涘缓绱㈠紩澶辫触: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("鍒涘缓绱㈠紩澶辫触: %s", body)
	}
	return nil
}

// DeleteIndex 鍒犻櫎绱㈠紩
func (c *ESClient) DeleteIndex(ctx context.Context, index string) error {
	req := esapi.IndicesDeleteRequest{Index: []string{index}}
	res, err := req.Do(ctx, c.client)
	if err != nil {
		return fmt.Errorf("鍒犻櫎绱㈠紩澶辫触: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("鍒犻櫎绱㈠紩澶辫触: %s", body)
	}
	return nil
}


