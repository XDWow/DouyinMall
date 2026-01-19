package producer

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/search/domain"
)

// rowMap 是 binlog 事件中的一行数据（map[columnName]value）
func parseRowToSyncEvent(rowMap map[string]interface{}, action domain.EventAction) (domain.SyncEvent, error) {
	event := domain.SyncEvent{
		Type:   domain.EventTypeProduct,
		Action: action,
	}

	id, err := parseInt64FromRow(rowMap, "id")
	if err != nil {
		return event, err
	}
	event.ID = id

	if action == domain.EventActionDelete {
		return event, nil
	}

	// CREATE/UPDATE 操作需要完整数据
	doc := &domain.ProductDocument{
		ID: id,
	}

	// Name
	if name, ok := rowMap["name"].(string); ok {
		doc.Name = name
	}

	// Price
	if price, err := parseInt64FromRow(rowMap, "price"); err == nil {
		doc.Price = price
	}

	// MerchantID
	if merchantID, err := parseInt64FromRow(rowMap, "merchant_id"); err == nil {
		doc.MerchantID = merchantID
	}

	// Description（可能为 NULL）
	if desc, ok := rowMap["description"].(string); ok && desc != "" {
		doc.Description = desc
	}

	// Picture（可能为 NULL）
	if pic, ok := rowMap["picture"].(string); ok && pic != "" {
		doc.Picture = pic
	}

	// SlideImgs（JSON 字符串 → []string）
	if slideImgsStr, ok := rowMap["slide_imgs"].(string); ok && slideImgsStr != "" {
		var slideImgs []string
		if err := json.Unmarshal([]byte(slideImgsStr), &slideImgs); err == nil {
			doc.SliderImgs = slideImgs
		}
	}

	// Categories（JSON 字符串 → []string）
	if categoriesStr, ok := rowMap["categories"].(string); ok && categoriesStr != "" {
		var categories []string
		if err := json.Unmarshal([]byte(categoriesStr), &categories); err == nil {
			doc.Categories = categories
		}
	}

	// MerchantName（可能为 NULL）
	if merchantName, ok := rowMap["merchant_name"].(string); ok && merchantName != "" {
		doc.MerchantName = merchantName
	}

	// CreatedAt（time.Time → Unix 时间戳）
	if createdAt, ok := rowMap["created_at"].(time.Time); ok {
		doc.CreatedTime = createdAt.Unix()
	} else if createdAtStr, ok := rowMap["created_at"].(string); ok {
		if t, err := time.Parse("2006-01-02 15:04:05", createdAtStr); err == nil {
			doc.CreatedTime = t.Unix()
		}
	}

	// UpdatedAt（time.Time → Unix 时间戳）
	if updatedAt, ok := rowMap["updated_at"].(time.Time); ok {
		doc.UpdatedTime = updatedAt.Unix()
	} else if updatedAtStr, ok := rowMap["updated_at"].(string); ok {
		if t, err := time.Parse("2006-01-02 15:04:05", updatedAtStr); err == nil {
			doc.UpdatedTime = t.Unix()
		}
	}

	event.Product = doc
	return event, nil
}

func parseInt64FromRow(rowMap map[string]interface{}, key string) (int64, error) {
	if val, ok := rowMap[key].(int64); ok {
		return val, nil
	}
	if valStr, ok := rowMap[key].(string); ok {
		val, err := strconv.ParseInt(valStr, 10, 64)
		if err != nil {
			return 0, err
		}
		return val, nil
	}
	return 0, nil
}
