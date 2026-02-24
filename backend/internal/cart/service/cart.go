package service

import (
	"context"

	"github.com/XDWow/DouyinMall/backend/internal/cart/domain"
	"github.com/XDWow/DouyinMall/backend/internal/cart/repository"
)

type CartService interface {
	AddItems(ctx context.Context, userID int64, items []domain.CartItem) error
	DeleteItems(ctx context.Context, userID int64, productIDs []int64) error
	GetCart(ctx context.Context, userID int64) (domain.Cart, error)
	EmptyCart(ctx context.Context, userID int64) error
	ChangeQty(ctx context.Context, userID int64, item domain.CartItem) error
	IncrementQty(ctx context.Context, userID, productID int64) (newQuantity int64, err error)
	DecrementQty(ctx context.Context, userID, productID int64) (newQuantity int64, err error)
}

type cartService struct {
	repo repository.CartRepository
}

func NewCartService(repo repository.CartRepository) CartService {
	return &cartService{
		repo: repo,
	}
}

func (s *cartService) AddItems(ctx context.Context, userID int64, items []domain.CartItem) error {
	return s.repo.AddItems(ctx, userID, items)
}

func (s *cartService) DeleteItems(ctx context.Context, userID int64, productIDs []int64) error {
	return s.repo.DeleteItems(ctx, userID, productIDs)
}

func (s *cartService) GetCart(ctx context.Context, userID int64) (domain.Cart, error) {
	return s.repo.GetCart(ctx, userID)
}

func (s *cartService) EmptyCart(ctx context.Context, userID int64) error {
	return s.repo.EmptyCart(ctx, userID)
}

func (s *cartService) ChangeQty(ctx context.Context, userID int64, item domain.CartItem) error {
	return s.repo.ChangeQty(ctx, userID, item)
}

func (s *cartService) IncrementQty(ctx context.Context, userID, productID int64) (newQuantity int64, err error) {
	return s.repo.IncrementQty(ctx, userID, productID)
}

func (s *cartService) DecrementQty(ctx context.Context, userID, productID int64) (newQuantity int64, err error) {
	return s.repo.DecrementQty(ctx, userID, productID)
}
