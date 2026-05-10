package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/seckill/domain"
	"github.com/stretchr/testify/require"
)

func TestSubmitUseCaseReturnsProcessingOnReserveSuccess(t *testing.T) {
	cache := &submitCacheStub{}
	producer := &submitProducerStub{
		result: &domain.Result{
			RequestNo: "1001",
			Status:    domain.RequestStatusProcessing,
		},
	}
	uc := NewSubmitUseCase(&submitActivityRepoStub{}, nil, cache, domain.NewNopSoldOutMarker(), producer, submitIDGenStub("1001"))

	result, err := uc.Execute(context.Background(), SubmitCmd{
		ActivityID: 1,
		UserID:     9,
	})

	require.NoError(t, err)
	require.Equal(t, domain.RequestStatusProcessing, result.Status)
	require.EqualValues(t, 24*time.Hour/time.Second+60, producer.lastUserTTL)
	require.Equal(t, "1001", producer.lastEvent.RequestNo)
}

func TestSubmitUseCaseReturnsDuplicateResultFromProducer(t *testing.T) {
	cache := &submitCacheStub{}
	producer := &submitProducerStub{
		result: &domain.Result{
			RequestNo:  "1002",
			Status:     domain.RequestStatusFailed,
			FailReason: domain.FailReasonDuplicate,
		},
		err: domain.ErrDuplicateSeckill,
	}
	uc := NewSubmitUseCase(&submitActivityRepoStub{}, nil, cache, domain.NewNopSoldOutMarker(), producer, submitIDGenStub("1002"))

	result, err := uc.Execute(context.Background(), SubmitCmd{
		ActivityID: 1,
		UserID:     9,
	})

	require.ErrorIs(t, err, domain.ErrDuplicateSeckill)
	require.Equal(t, domain.RequestStatusFailed, result.Status)
	require.Equal(t, domain.FailReasonDuplicate, result.FailReason)
}

func TestSubmitUseCaseReturnsProducerError(t *testing.T) {
	cache := &submitCacheStub{}
	producer := &submitProducerStub{err: errors.New("send failed")}
	uc := NewSubmitUseCase(&submitActivityRepoStub{}, nil, cache, domain.NewNopSoldOutMarker(), producer, submitIDGenStub("1003"))

	result, err := uc.Execute(context.Background(), SubmitCmd{
		ActivityID: 1,
		UserID:     9,
	})

	require.Nil(t, result)
	require.EqualError(t, err, "send failed")
}

func TestSubmitUseCaseShortCircuitsWhenLocalSoldOutMarked(t *testing.T) {
	cache := &submitCacheStub{}
	producer := &submitProducerStub{}
	soldOut := &submitSoldOutMarkerStub{soldOut: map[int64]bool{1: true}}
	uc := NewSubmitUseCase(&submitActivityRepoStub{}, nil, cache, soldOut, producer, submitIDGenStub("1004"))

	result, err := uc.Execute(context.Background(), SubmitCmd{
		ActivityID: 1,
		UserID:     9,
	})

	require.ErrorIs(t, err, domain.ErrOutOfStock)
	require.NotNil(t, result)
	require.Equal(t, domain.RequestStatusFailed, result.Status)
	require.Equal(t, domain.FailReasonOutOfStock, result.FailReason)
	require.False(t, producer.called)
}

type submitActivityRepoStub struct{}

func (s *submitActivityRepoStub) Create(context.Context, *domain.Activity) error { return nil }

func (s *submitActivityRepoStub) FindByID(context.Context, int64) (*domain.Activity, error) {
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

func (s *submitActivityRepoStub) UpdateStatus(context.Context, int64, string) error { return nil }

type submitCacheStub struct{}

func (s *submitCacheStub) SetActivity(context.Context, *domain.Activity) error { return nil }

func (s *submitCacheStub) GetActivity(context.Context, int64) (*domain.Activity, error) {
	return nil, nil
}

func (s *submitCacheStub) SetActivityStock(context.Context, int64, int32) error { return nil }

func (s *submitCacheStub) AtomicReserve(context.Context, int64, int64, string, int64) (int64, error) {
	return 0, nil
}

func (s *submitCacheStub) Compensate(context.Context, int64, int64, string, domain.Result) error {
	return nil
}

func (s *submitCacheStub) SetResult(context.Context, domain.Result) error { return nil }

func (s *submitCacheStub) GetResult(context.Context, string) (*domain.Result, error) { return nil, nil }

func (s *submitCacheStub) ResolveTransaction(context.Context, int64, int64, string) (domain.TransactionResolution, error) {
	return domain.TransactionResolutionUnknown, nil
}

type submitProducerStub struct {
	result      *domain.Result
	err         error
	lastEvent   domain.Event
	lastUserTTL int64
	called      bool
}

func (s *submitProducerStub) Submit(_ context.Context, evt domain.Event, userTTLSeconds int64) (*domain.Result, error) {
	s.called = true
	s.lastEvent = evt
	s.lastUserTTL = userTTLSeconds
	return s.result, s.err
}

type submitIDGenStub string

func (s submitIDGenStub) GenerateID() string { return string(s) }

type submitSoldOutMarkerStub struct {
	soldOut map[int64]bool
}

func (s *submitSoldOutMarkerStub) IsSoldOut(activityID int64) bool {
	return s.soldOut[activityID]
}

func (s *submitSoldOutMarkerStub) MarkSoldOut(activityID int64) {
	if s.soldOut == nil {
		s.soldOut = make(map[int64]bool)
	}
	s.soldOut[activityID] = true
}

func (s *submitSoldOutMarkerStub) Clear(activityID int64) {
	if s.soldOut == nil {
		return
	}
	delete(s.soldOut, activityID)
}
