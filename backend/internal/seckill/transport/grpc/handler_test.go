package grpc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/seckill/domain"
	"github.com/XDWow/DouyinMall/backend/internal/seckill/usecase"
	seckillv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/seckill/v1"
	"github.com/stretchr/testify/require"
)

func TestSubmitSeckillReturnsBusinessFailureInResponse(t *testing.T) {
	submitUC := usecase.NewSubmitUseCase(
		&grpcSubmitActivityRepoStub{},
		nil,
		&grpcSubmitCacheStub{},
		domain.NewNopSoldOutMarker(),
		&grpcSubmitProducerStub{
			result: &domain.Result{
				RequestNo:  "req-1",
				Status:     domain.RequestStatusFailed,
				FailReason: domain.FailReasonDuplicate,
			},
			err: domain.ErrDuplicateSeckill,
		},
		grpcSubmitIDGen("req-1"),
	)
	h := &Handler{submitUC: submitUC}

	resp, err := h.SubmitSeckill(context.Background(), &seckillv1.SubmitSeckillReq{
		ActivityId: 1,
		UserId:     9,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, "req-1", resp.GetRequestNo())
	require.Equal(t, domain.RequestStatusFailed, resp.GetStatus())
	require.Equal(t, domain.FailReasonDuplicate, resp.GetMessage())
}

func TestSubmitSeckillReturnsTransportErrorWhenSubmitFails(t *testing.T) {
	submitUC := usecase.NewSubmitUseCase(
		&grpcSubmitActivityRepoStub{},
		nil,
		&grpcSubmitCacheStub{},
		domain.NewNopSoldOutMarker(),
		&grpcSubmitProducerStub{err: errors.New("send failed")},
		grpcSubmitIDGen("req-2"),
	)
	h := &Handler{submitUC: submitUC}

	resp, err := h.SubmitSeckill(context.Background(), &seckillv1.SubmitSeckillReq{
		ActivityId: 1,
		UserId:     9,
	})

	require.Nil(t, resp)
	require.EqualError(t, err, "send failed")
}

type grpcSubmitActivityRepoStub struct{}

func (s *grpcSubmitActivityRepoStub) Create(context.Context, *domain.Activity) error { return nil }

func (s *grpcSubmitActivityRepoStub) FindByID(context.Context, int64) (*domain.Activity, error) {
	return &domain.Activity{
		ID:           1,
		ProductID:    11,
		SKUID:        12,
		SeckillPrice: 99,
		Status:       domain.ActivityStatusOnline,
		StartTime:    time.Now().Add(-time.Minute),
		EndTime:      time.Now().Add(time.Minute),
	}, nil
}

func (s *grpcSubmitActivityRepoStub) UpdateStatus(context.Context, int64, string) error { return nil }

type grpcSubmitCacheStub struct{}

func (s *grpcSubmitCacheStub) SetActivity(context.Context, *domain.Activity) error { return nil }

func (s *grpcSubmitCacheStub) GetActivity(context.Context, int64) (*domain.Activity, error) {
	return nil, nil
}

func (s *grpcSubmitCacheStub) SetActivityStock(context.Context, int64, int32) error { return nil }

func (s *grpcSubmitCacheStub) AtomicReserve(context.Context, int64, int64, string, int64) (int64, error) {
	return 0, nil
}

func (s *grpcSubmitCacheStub) Compensate(context.Context, int64, int64, string, domain.Result) error {
	return nil
}

func (s *grpcSubmitCacheStub) SetResult(context.Context, domain.Result) error { return nil }

func (s *grpcSubmitCacheStub) GetResult(context.Context, string) (*domain.Result, error) {
	return nil, nil
}

func (s *grpcSubmitCacheStub) ResolveTransaction(context.Context, int64, int64, string) (domain.TransactionResolution, error) {
	return domain.TransactionResolutionUnknown, nil
}

type grpcSubmitProducerStub struct {
	result *domain.Result
	err    error
}

func (s *grpcSubmitProducerStub) Submit(context.Context, domain.Event, int64) (*domain.Result, error) {
	return s.result, s.err
}

type grpcSubmitIDGen string

func (s grpcSubmitIDGen) GenerateID() string { return string(s) }
