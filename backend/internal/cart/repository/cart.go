package repository

import (
	"context"
	"github.com/XDWow/DouyinMall/backend/internal/cart/domain"
)

type CartRepository interface {
	AddItem(ctx context.Context, userID int64, item domain.CartItem) error
	DeleteItem(ctx context.Context, userID, itemID int64) error
	GetCart(ctx context.Context, userID int64) (domain.Cart, error)
	EmptyCart(ctx context.Context, userID int64) error
	ChangeQty(ctx context.Context, userID, itemID int64, quantity int) error
	IncreaseQty(ctx context.Context, userID, itemID int64) error
	DecreaseQty(ctx context.Context, userID, itemID int64) error
}

type CachedCartRepository struct {
	cache
}
