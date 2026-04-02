package domain

import "context"

type RequestRepository interface {
	Create(ctx context.Context, request *Request) error
	FindOpenByUserOrder(ctx context.Context, userID, orderID int64, requestType RequestType) (*Request, error)
	GetByRequestNo(ctx context.Context, requestNo string) (*Request, error)
}
