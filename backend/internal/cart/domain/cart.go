package domain

import (
	"time"
)

type Cart struct {
	ID int64 `json:"id"`
	UserID int64 `json:"user_id"`
	Items []CartItem `json:"items"`
}

type CartItem struct {
	ProductID int64 `json:"product_id"`
	Quantity int64 `json:"quantity"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}