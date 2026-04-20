package domain

import "time"

type SessionMessage struct {
	ID         string
	SessionID  string
	Role       Role
	Content    string
	Intent     Intent
	Confidence float64
	CreatedAt  time.Time
}

type Message = SessionMessage

// Session only keeps reusable cross-turn references.
type Session struct {
	SessionID        string         `json:"session_id"`
	UserID           int64          `json:"user_id"`
	RecentMessages   []MessageTurn  `json:"recent_messages,omitempty"`
	CurrentOrder     string         `json:"current_order,omitempty"`
	CurrentProduct   string         `json:"current_product,omitempty"`
	CurrentSpec      string         `json:"current_spec,omitempty"`
	CurrentPromotion string         `json:"current_promotion,omitempty"`
	ProductList      []string       `json:"product_list,omitempty"`
	OrderList        []string       `json:"order_list,omitempty"`
	PromotionList    []string       `json:"promotion_list,omitempty"`
	Slots            map[string]any `json:"-"`
}

func (s *Session) ApplyPersistedFields(src Session) {
	if s == nil {
		return
	}
	s.SessionID = src.SessionID
	s.UserID = src.UserID
	s.RecentMessages = append([]MessageTurn(nil), src.RecentMessages...)
	s.CurrentOrder = src.CurrentOrder
	s.CurrentProduct = src.CurrentProduct
	s.CurrentSpec = src.CurrentSpec
	s.CurrentPromotion = src.CurrentPromotion
	s.ProductList = append([]string(nil), src.ProductList...)
	s.OrderList = append([]string(nil), src.OrderList...)
	s.PromotionList = append([]string(nil), src.PromotionList...)
	s.Slots = CloneAnyMap(src.Slots)
}
