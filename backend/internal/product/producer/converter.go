package producer

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/search/domain"
)

func parseRowToSyncEvent(rowMap map[string]interface{}, action domain.EventAction) (domain.SyncEvent, error) {
	event := domain.SyncEvent{
		Type:   domain.EventTypeProduct,
		Action: action,
	}

	id, err := parseRequiredInt64FromRow(rowMap, "id")
	if err != nil {
		return event, err
	}
	event.ID = id

	if action == domain.EventActionDelete {
		return event, nil
	}

	doc := &domain.ProductDocument{ID: id}

	if name, ok := rowMap["name"].(string); ok {
		doc.Name = name
	}
	if price, err := parseInt64FromRow(rowMap, "price"); err == nil {
		doc.Price = price
	}
	if merchantID, err := parseInt64FromRow(rowMap, "merchant_id"); err == nil {
		doc.MerchantID = merchantID
	}
	if desc, ok := rowMap["description"].(string); ok && desc != "" {
		doc.Description = desc
	}
	if pic, ok := rowMap["picture"].(string); ok && pic != "" {
		doc.Picture = pic
	}
	if slideImgsStr, ok := rowMap["slide_imgs"].(string); ok && slideImgsStr != "" {
		var slideImgs []string
		if err := json.Unmarshal([]byte(slideImgsStr), &slideImgs); err == nil {
			doc.SliderImgs = slideImgs
		}
	}
	if categoriesStr, ok := rowMap["categories"].(string); ok && categoriesStr != "" {
		var categories []string
		if err := json.Unmarshal([]byte(categoriesStr), &categories); err == nil {
			doc.Categories = categories
		}
	}
	if merchantName, ok := rowMap["merchant_name"].(string); ok && merchantName != "" {
		doc.MerchantName = merchantName
	}
	doc.CreatedTime = parseUnixTime(rowMap["created_at"])
	doc.UpdatedTime = parseUnixTime(rowMap["updated_at"])

	event.Product = doc
	return event, nil
}

func parseRequiredInt64FromRow(rowMap map[string]interface{}, key string) (int64, error) {
	val, err := parseInt64FromRow(rowMap, key)
	if err != nil {
		return 0, err
	}
	if val == 0 {
		return 0, fmt.Errorf("%s is required", key)
	}
	return val, nil
}

func parseInt64FromRow(rowMap map[string]interface{}, key string) (int64, error) {
	switch val := rowMap[key].(type) {
	case int64:
		return val, nil
	case int32:
		return int64(val), nil
	case int:
		return int64(val), nil
	case uint64:
		return int64(val), nil
	case []byte:
		return strconv.ParseInt(string(val), 10, 64)
	case string:
		if val == "" {
			return 0, nil
		}
		return strconv.ParseInt(val, 10, 64)
	default:
		return 0, nil
	}
}

func parseUnixTime(value interface{}) int64 {
	switch v := value.(type) {
	case time.Time:
		return v.Unix()
	case string:
		if t, err := time.Parse("2006-01-02 15:04:05", v); err == nil {
			return t.Unix()
		}
	case []byte:
		if t, err := time.Parse("2006-01-02 15:04:05", string(v)); err == nil {
			return t.Unix()
		}
	}
	return 0
}
